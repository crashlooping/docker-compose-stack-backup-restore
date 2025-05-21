package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/archive"
	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/docker"
)

const (
	backupTarGzPattern = "backup_%s_%s.tar.gz"
	backupZipPattern   = "backup_%s_%s.zip"
)

func BackupComposeStack(srcPath, dstPath string) error {
	// Check permissions before stopping stack or backing up
	err := archive.CheckDirReadable(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[permission error] Some files or directories in '%s' are not readable.\n%s\n", srcPath, err)
		fmt.Fprintln(os.Stderr, "You may need to run this tool with elevated permissions (e.g., 'sudo'). Backup aborted.")
		return err
	}

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

	volumeTarballs, err := exportAllComposeVolumes(srcPath, composeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[debug] exportAllComposeVolumes error: %v\n", err)
		return err
	}

	folderName := filepath.Base(srcPath)
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf(backupTarGzPattern, folderName, timestamp)
	backupPath := filepath.Join(dstPath, backupName)
	fmt.Printf("Creating tar.gz backup: %s\n", backupPath)
	err = archive.TarGzFolderWithVolumes(srcPath, backupPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("tar.gz backup created.")

	zipName := fmt.Sprintf(backupZipPattern, folderName, timestamp)
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

func BackupComposeStackWithFormats(srcPath, dstPath string, formats []string, password string) error {
	if len(formats) == 0 {
		return nil
	}
	// Check permissions before stopping stack or backing up
	err := archive.CheckDirReadable(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[permission error] Some files or directories in '%s' are not readable.\n%s\n", srcPath, err)
		fmt.Fprintln(os.Stderr, "You may need to run this tool with elevated permissions (e.g., 'sudo'). Backup aborted.")
		return err
	}

	composeFile, err := docker.FindComposeFile(srcPath)
	if err != nil {
		return err
	}
	docker.PrintComposeFileStatus(composeFile)
	stackWasRunning, err := docker.StopStackIfRunning(srcPath, composeFile)
	if err != nil {
		return err
	}

	volumeTarballs, err := exportAllComposeVolumes(srcPath, composeFile)
	if err != nil {
		return err
	}

	folderName := filepath.Base(srcPath)
	timestamp := time.Now().Format("20060102_150405")

	jobs := makeArchiveJobs(formats, srcPath, dstPath, folderName, timestamp, volumeTarballs)
	if err := runArchiveJobs(jobs); err != nil {
		return err
	}

	// Encrypt each backup file if password is set
	if password != "" {
		for _, format := range formats {
			var backupName string
			switch format {
			case "tar.gz":
				backupName = fmt.Sprintf(backupTarGzPattern, folderName, timestamp)
			case "zip":
				backupName = fmt.Sprintf(backupZipPattern, folderName, timestamp)
			}
			backupPath := filepath.Join(dstPath, backupName)
			encPath := backupPath + ".enc"
			fmt.Printf("Encrypting %s -> %s\n", backupPath, encPath)
			err := archive.EncryptFile(backupPath, encPath, password)
			if err != nil {
				return fmt.Errorf("failed to encrypt backup: %w", err)
			}
			os.Remove(backupPath)
		}
	}

	cleanupTempFiles(volumeTarballs)

	if err := restartStackIfNeeded(stackWasRunning, srcPath, composeFile); err != nil {
		return err
	}
	return nil
}

func makeArchiveJobs(formats []string, srcPath, dstPath, folderName, timestamp string, volumeTarballs []string) []func() error {
	var jobs []func() error
	for _, format := range formats {
		switch format {
		case "tar.gz":
			backupName := fmt.Sprintf(backupTarGzPattern, folderName, timestamp)
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
			zipName := fmt.Sprintf(backupZipPattern, folderName, timestamp)
			zipPath := filepath.Join(dstPath, zipName)
			jobs = append(jobs, func() error {
				fmt.Printf("Creating zip backup: %s\n", zipPath)
				err := archive.ZipFolderWithVolumes(srcPath, zipPath, volumeTarballs)
				if err == nil {
					fmt.Println("zip backup created.")
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
		fmt.Printf("Removing temp file: %s\n", f)
		os.Remove(f)
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
