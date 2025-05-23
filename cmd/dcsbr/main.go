package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/backup"
	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) == 1 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Println(`docker-compose-stack-backup-restore

Usage:
  dcsbr.exe backup
    Run backup for all stacks defined in config.yaml.

  dcsbr.exe restore --target <restore-folder> <backup-archive>
    Restore a backup archive (.tar.gz, .zip, or .enc) to the target folder.

  dcsbr.exe decrypt --target <target-folder> <backup-archive.enc>
    Decrypt an encrypted backup file to the target folder (no extraction).

  dcsbr.exe verify
    Print and verify the config.yaml file, masking the password field.

Options:
  --help, -h   Show this help message.

See README.md for more details and configuration examples.`)
		return
	}

	if os.Args[1] == "backup" {
		cfg, err := backup.LoadConfig("config.yaml")
		if err != nil {
			fmt.Println("Error loading config.yaml:", err)
			os.Exit(1)
		}
		for _, srcPath := range cfg.Backup.Sources {
			absSrc, _ := filepath.Abs(srcPath)
			absDst, _ := filepath.Abs(cfg.Backup.Target)
			fmt.Printf("Starting backup of '%s' to '%s' (formats: %v)...\n", absSrc, absDst, cfg.Backup.Formats)
			if cfg.Backup.Password != "" {
				fmt.Println("[encryption] Password is set. Encrypted backups will be created.")
			} else {
				fmt.Println("[encryption] No password set. Backups will NOT be encrypted.")
			}
			err := backup.BackupComposeStackWithFormats(absSrc, absDst, cfg.Backup.Formats, cfg.Backup.Password, cfg.Backup.MaxBackups, cfg.Backup.Prefix)
			if err != nil {
				fmt.Printf("Error backing up %s: %v\n", absSrc, err)
			}
		}
		fmt.Println("All backups completed.")
		return
	} else if len(os.Args) > 1 && os.Args[1] == "restore" {
		restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
		target := restoreCmd.String("target", ".", "Target directory for stack restore")
		restoreCmd.Parse(os.Args[2:])
		if restoreCmd.NArg() < 1 {
			fmt.Println("Usage: dcsbr restore [--target DIR] <backup-archive>")
			os.Exit(1)
		}
		cfg, err := backup.LoadConfig("config.yaml")
		if err != nil {
			fmt.Println("Error loading config.yaml:", err)
			os.Exit(1)
		}
		archivePath := restoreCmd.Arg(0)
		stackName := extractStackNameFromArchive(archivePath, cfg.Backup.Prefix)
		resolvedTarget := filepath.Join(*target, stackName)
		if err := os.MkdirAll(resolvedTarget, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating target folder: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nRestore will extract from:\n  Source archive: %s\n  Target folder:  %s\n", archivePath, resolvedTarget)
		fmt.Print("\nAre you sure you want to proceed? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Restore cancelled.")
			os.Exit(0)
		}
		opts := backup.RestoreOptions{TargetDir: resolvedTarget}
		// If encrypted, pass password from config if present
		if strings.HasSuffix(archivePath, ".enc") {
			cfg, _ := backup.LoadConfig("config.yaml")
			if cfg != nil && cfg.Backup.Password != "" {
				opts.Password = cfg.Backup.Password
			}
		}
		if err := backup.RestoreFromBackup(archivePath, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Restore failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Restore completed successfully.")
		return
	} else if len(os.Args) > 1 && os.Args[1] == "decrypt" {
		decryptCmd := flag.NewFlagSet("decrypt", flag.ExitOnError)
		target := decryptCmd.String("target", ".", "Target directory for decrypted file")
		decryptCmd.Parse(os.Args[2:])
		if decryptCmd.NArg() < 1 {
			fmt.Println("Usage: dcsbr decrypt --target DIR <backup-archive.enc>")
			os.Exit(1)
		}
		cfg, err := backup.LoadConfig("config.yaml")
		if err != nil {
			fmt.Println("Error loading config.yaml:", err)
			os.Exit(1)
		}
		encPath := decryptCmd.Arg(0)
		password := ""
		if cfg != nil && cfg.Backup.Password != "" {
			password = cfg.Backup.Password
		} else {
			fmt.Print("Enter password to decrypt backup: ")
			var pw string
			fmt.Scanln(&pw)
			password = strings.TrimSpace(pw)
		}
		outName := filepath.Base(encPath[:len(encPath)-4])
		if strings.HasPrefix(outName, cfg.Backup.Prefix+"_backup_") {
			outName = outName[len(cfg.Backup.Prefix+"_backup_"):]
		}
		outPath := filepath.Join(*target, cfg.Backup.Prefix+"_backup_"+outName)
		err = backup.DecryptBackupFile(encPath, outPath, password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Decryption failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Decryption completed successfully.")
		return
	} else if len(os.Args) > 1 && (os.Args[1] == "verify" || os.Args[1] == "--verify") {
		cfg, err := backup.LoadConfig("config.yaml")
		if err != nil {
			fmt.Println("Error loading config.yaml:", err)
			os.Exit(1)
		}
		// Mask the password
		maskedCfg := *cfg
		if maskedCfg.Backup.Password != "" {
			maskedCfg.Backup.Password = strings.Repeat("*", len(maskedCfg.Backup.Password))
		}
		out, err := yaml.Marshal(&maskedCfg)
		if err != nil {
			fmt.Println("Error marshaling config:", err)
			os.Exit(1)
		}
		fmt.Println("Config loaded and verified:")
		fmt.Println(string(out))
		fmt.Println("Config verification successful.")
		return
	}
}

func extractStackNameFromArchive(archivePath string, prefix string) string {
	base := filepath.Base(archivePath)
	re := regexp.MustCompile(fmt.Sprintf(`^%s_backup_(.+?)_\d{8}_\d{6}\.(tar\.gz|zip)$`, regexp.QuoteMeta(prefix)))
	matches := re.FindStringSubmatch(base)
	if len(matches) > 1 {
		return matches[1]
	}
	// fallback: remove extension and prefix_backup_ prefix
	name := base
	if strings.HasPrefix(name, prefix+"_backup_") {
		name = name[len(prefix+"_backup_"):]
	}
	if idx := strings.Index(name, "_"); idx > 0 {
		name = name[:idx]
	}
	return name
}
