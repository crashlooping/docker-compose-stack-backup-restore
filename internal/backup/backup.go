package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/archive"
	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/docker"
)

const (
	backupTarGzPattern = "%s_backup_%s_%s.tar.gz"
	backupZipPattern   = "%s_backup_%s_%s.zip"
	backupZstPattern   = "%s_backup_%s_%s.tar.zst"
)

func BackupComposeStack(srcPath, dstPath string, prefix string) error {
	composeFile, err := docker.FindComposeFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[debug] FindComposeFile error: %v\n", err)
		return err
	}
	docker.PrintComposeFileStatus(composeFile)
	stackWasRunning, err := docker.StopStackIfRunning(srcPath, composeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[debug] StopStackIfRunning error: %v\n", err)
		return err
	}

	err = archive.CheckDirReadable(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[permission error] Some files or directories in '%s' are not readable.\n%s\n", srcPath, err)
		fmt.Fprintln(os.Stderr, "You may need to run this tool with elevated permissions (e.g., 'sudo'). Backup aborted.")
		return err
	}

	volumeTarballs, err := exportAllComposeVolumes(srcPath, composeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[debug] exportAllComposeVolumes error: %v\n", err)
		return err
	}

	folderName := filepath.Base(srcPath)
	timestamp := time.Now().Format("20060102_150405.000000000")
	backupName := fmt.Sprintf(backupTarGzPattern, prefix, folderName, timestamp)
	backupPath := filepath.Join(dstPath, backupName)
	fmt.Printf("Creating tar.gz backup: %s\n", backupPath)
	err = archive.TarGzFolderWithVolumes(srcPath, backupPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("tar.gz backup created.")

	zipName := fmt.Sprintf(backupZipPattern, prefix, folderName, timestamp)
	zipPath := filepath.Join(dstPath, zipName)
	fmt.Printf("Creating zip backup: %s\n", zipPath)
	err = archive.ZipFolderWithVolumes(srcPath, zipPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("zip backup created.")

	for _, f := range volumeTarballs {
		fmt.Printf("Removing temp file: %s\n", f)
		os.Remove(f)
	}

	if stackWasRunning {
		fmt.Println("Restarting stack...")
		err = docker.ComposeUp(srcPath, composeFile)
		if err != nil {
			return err
		}
		fmt.Println("Stack restarted.")
	}
	return nil
}

// BackupResult reports whether a stack was running before backup and which
// compose file was used, so the caller can decide whether/how to restart it.
type BackupResult struct {
	WasRunning  bool
	ComposeFile string
}

func BackupComposeStackWithFormats(srcPath, dstPath string, formats []string, password string, maxBackups int, prefix string) error {
	_, err := BackupComposeStackWithFormatsCore(srcPath, dstPath, formats, password, maxBackups, prefix, true)
	return err
}

// BackupComposeStackWithFormatsDeferred backs up a stack but does NOT restart
// it afterwards. The caller is responsible for restarting it later (e.g. after
// all other stacks have been backed up). Returns the pre-backup state.
func BackupComposeStackWithFormatsDeferred(srcPath, dstPath string, formats []string, password string, maxBackups int, prefix string) (*BackupResult, error) {
	return BackupComposeStackWithFormatsCore(srcPath, dstPath, formats, password, maxBackups, prefix, false)
}

func BackupComposeStackWithFormatsCore(srcPath, dstPath string, formats []string, password string, maxBackups int, prefix string, restart bool) (*BackupResult, error) {
	result := &BackupResult{}
	if len(formats) == 0 {
		return result, nil
	}
	composeFile, err := docker.FindComposeFile(srcPath)
	if err != nil {
		return result, err
	}
	docker.PrintComposeFileStatus(composeFile)
	stackWasRunning, err := docker.StopStackIfRunning(srcPath, composeFile)
	if err != nil {
		return result, err
	}
	result.WasRunning = stackWasRunning
	result.ComposeFile = composeFile

	err = archive.CheckDirReadable(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[permission error] Some files or directories in '%s' are not readable.\n%s\n", srcPath, err)
		fmt.Fprintln(os.Stderr, "You may need to run this tool with elevated permissions (e.g., 'sudo'). Backup aborted.")
		return result, err
	}

	volumeTarballs, err := exportAllComposeVolumes(srcPath, composeFile)
	if err != nil {
		return result, err
	}

	folderName := filepath.Base(srcPath)
	timestamp := time.Now().Format("20060102_150405.000000000")

	jobs := makeArchiveJobs(formats, srcPath, dstPath, folderName, timestamp, volumeTarballs, prefix)
	if err := runArchiveJobs(jobs); err != nil {
		return result, err
	}

	// Encrypt each backup file if password is set
	if password != "" {
		for _, format := range formats {
			var backupName string
			switch format {
			case "tar.gz":
				backupName = fmt.Sprintf(backupTarGzPattern, prefix, folderName, timestamp)
			case "zip":
				backupName = fmt.Sprintf(backupZipPattern, prefix, folderName, timestamp)
			case "zst":
				backupName = fmt.Sprintf(backupZstPattern, prefix, folderName, timestamp)
			}
			backupPath := filepath.Join(dstPath, backupName)
			encPath := backupPath + ".enc"
			fmt.Printf("Encrypting %s -> %s\n", backupPath, encPath)
			err := archive.EncryptFile(backupPath, encPath, password)
			if err != nil {
				return result, fmt.Errorf("failed to encrypt backup: %w", err)
			}
			if err := os.Remove(backupPath); err != nil {
				fmt.Fprintf(os.Stderr, "[cleanup] Failed to remove unencrypted backup %s: %v\n", backupPath, err)
			}
		}
	}

	cleanupBackupsAfterRun(dstPath, folderName, password != "", maxBackups, prefix)

	cleanupTempFiles(volumeTarballs)

	if restart {
		if err := restartStackIfNeeded(stackWasRunning, srcPath, composeFile); err != nil {
			return result, err
		}
	}
	return result, nil
}

// BackupAllSources backs up every source in the config. Sources listed in
// sources_first_last are stopped and backed up FIRST, then every regular
// source is backed up (restarting each as usual), and finally the
// first/last sources are restarted LAST. This keeps monitoring tools from
// reporting the other stacks going down during the backup run.
func BackupAllSources(cfg *Config) error {
	var deferred []*BackupResult
	// 1. Stop + back up first/last sources, but leave them stopped.
	for _, srcPath := range cfg.Backup.SourcesFirstLast {
		absSrc, _ := filepath.Abs(srcPath)
		absDst, _ := filepath.Abs(cfg.Backup.Target)
		fmt.Printf("Starting backup of '%s' to '%s' (formats: %v)...\n", absSrc, absDst, cfg.Backup.Formats)
		result, err := BackupComposeStackWithFormatsDeferred(absSrc, absDst, cfg.Backup.Formats, cfg.Backup.Password, cfg.Backup.MaxBackups, cfg.Backup.Prefix)
		if err != nil {
			fmt.Printf("Error backing up %s: %v\n", absSrc, err)
		}
		deferred = append(deferred, result)
	}
	// 2. Back up every regular source, restarting each as usual.
	for _, srcPath := range cfg.Backup.Sources {
		absSrc, _ := filepath.Abs(srcPath)
		absDst, _ := filepath.Abs(cfg.Backup.Target)
		fmt.Printf("Starting backup of '%s' to '%s' (formats: %v)...\n", absSrc, absDst, cfg.Backup.Formats)
		err := BackupComposeStackWithFormats(absSrc, absDst, cfg.Backup.Formats, cfg.Backup.Password, cfg.Backup.MaxBackups, cfg.Backup.Prefix)
		if err != nil {
			fmt.Printf("Error backing up %s: %v\n", absSrc, err)
		}
	}
	// 3. Restart the first/last sources that were running before backup.
	for i := len(deferred) - 1; i >= 0; i-- {
		result := deferred[i]
		if result.WasRunning {
			absSrc, _ := filepath.Abs(cfg.Backup.SourcesFirstLast[i])
			if err := restartStackIfNeeded(true, absSrc, result.ComposeFile); err != nil {
				fmt.Printf("Error restarting %s: %v\n", absSrc, err)
			}
		}
	}
	return nil
}

// cleanupBackupsAfterRun enforces maxBackups across ALL backup files for a stack,
// regardless of format (tar.gz, zip, zst, encrypted or not).
func cleanupBackupsAfterRun(dstPath, stackName string, encrypted bool, maxBackups int, prefix string) {
	fmt.Print("[retention] Checking max_backups across all formats...\n")
	// Glob all backup files for this stack regardless of extension or encryption
	patterns := []string{fmt.Sprintf("%s_backup_%s_*", prefix, stackName)}
	cleanupOldBackups(dstPath, patterns, maxBackups, stackName, prefix)
}

func makeArchiveJobs(formats []string, srcPath, dstPath, folderName, timestamp string, volumeTarballs []string, prefix string) []func() error {
	var jobs []func() error
	for _, format := range formats {
		switch format {
		case "tar.gz":
			backupName := fmt.Sprintf(backupTarGzPattern, prefix, folderName, timestamp)
			backupPath := filepath.Join(dstPath, backupName)
			jobs = append(jobs, func() error {
				fmt.Printf("Creating tar.gz backup: %s\n", backupPath)
				err := archive.TarGzFolderWithVolumes(srcPath, backupPath, volumeTarballs)
				if err == nil {
					fmt.Println("tar.gz backup created.")
				}
				return err
			})
		case "zip":
			zipName := fmt.Sprintf(backupZipPattern, prefix, folderName, timestamp)
			zipPath := filepath.Join(dstPath, zipName)
			jobs = append(jobs, func() error {
				fmt.Printf("Creating zip backup: %s\n", zipPath)
				err := archive.ZipFolderWithVolumes(srcPath, zipPath, volumeTarballs)
				if err == nil {
					fmt.Println("zip backup created.")
				}
				return err
			})
		case "zst":
			zstName := fmt.Sprintf(backupZstPattern, prefix, folderName, timestamp)
			zstPath := filepath.Join(dstPath, zstName)
			jobs = append(jobs, func() error {
				fmt.Printf("Creating tar.zst backup: %s\n", zstPath)
				err := archive.TarZstFolderWithVolumes(srcPath, zstPath, volumeTarballs)
				if err == nil {
					fmt.Println("tar.zst backup created.")
				}
				return err
			})
		}
	}
	return jobs
}

func runArchiveJobs(jobs []func() error) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(jobs))
	for _, job := range jobs {
		wg.Add(1)
		go func(j func() error) {
			defer wg.Done()
			errCh <- j()
		}(job)
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil {
			return e
		}
	}
	return nil
}

func cleanupTempFiles(files []string) {
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			fmt.Fprintf(os.Stderr, "[cleanup] Failed to remove temp file %s: %v\n", f, err)
		} else {
			fmt.Printf("Removing temp file: %s\n", f)
		}
	}
}

func restartStackIfNeeded(stackWasRunning bool, srcPath, composeFile string) error {
	if stackWasRunning {
		fmt.Println("Restarting stack...")
		err := docker.ComposeUp(srcPath, composeFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[restart error] Failed to restart stack: %v\n", err)
			return err
		}
		fmt.Println("Stack restarted.")
	}
	return nil
}

func cleanupOldBackups(dstPath string, patterns []string, maxBackups int, stackName, prefix string) error {
	if maxBackups == 0 {
		fmt.Printf("[retention] max_backups is 0 (unlimited), skipping pruning for %s\n", stackName)
		return nil
	}
	var files []string
	for _, pat := range patterns {
		matches, err := filepath.Glob(filepath.Join(dstPath, pat))
		if err != nil {
			return err
		}
		files = append(files, matches...)
	}
	fmt.Printf("[retention] Checking max_backups for %s: found %d backup files, max allowed is %d\n", stackName, len(files), maxBackups)
	if len(files) <= maxBackups {
		fmt.Printf("[retention] No pruning needed for %s\n", stackName)
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j] // reverse lexicographical, newest first
	})
	for _, f := range files[maxBackups:] {
		if err := os.Remove(f); err != nil {
			fmt.Fprintf(os.Stderr, "[retention] Failed to remove old backup %s: %v\n", f, err)
		} else {
			fmt.Printf("[retention] Removing old backup: %s\n", f)
		}
	}
	return nil
}

func exportAllComposeVolumes(srcPath, composeFile string) ([]string, error) {
	var volumeTarballs []string
	if composeFile == "" {
		return volumeTarballs, nil
	}
	fmt.Println("Detecting and exporting docker volumes...")
	volumes, err := docker.ListComposeVolumes(srcPath, composeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not list Docker volumes for stack at %s: %v\nContinuing backup without volumes.\n", srcPath, err)
		return volumeTarballs, nil
	}
	stackName := filepath.Base(srcPath)
	for _, v := range volumes {
		fullVolumeName := stackName + "_" + v
		mountPath := docker.GetVolumeMountPathFromCompose(composeFile, v, srcPath)
		tarPath, err := archive.ExportDockerVolumeTar(fullVolumeName, mountPath)
		if err != nil {
			fmt.Printf("Warning: could not export volume %s: %v\n", fullVolumeName, err)
			continue
		}
		fmt.Printf("Generated temp file: %s\n", tarPath)
		volumeTarballs = append(volumeTarballs, tarPath)
		fmt.Printf("Exported volume %s to %s\n", fullVolumeName, tarPath)
	}
	return volumeTarballs, nil
}

// DecryptBackupFile is a helper for CLI to decrypt a backup file using the archive package.
func DecryptBackupFile(encPath, outPath, password string) error {
	return archive.DecryptFile(encPath, outPath, password)
}
