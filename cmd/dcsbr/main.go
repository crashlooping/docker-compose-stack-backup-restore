package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/backup"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "restore" {
		restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
		target := restoreCmd.String("target", ".", "Target directory for stack restore")
		restoreCmd.Parse(os.Args[2:])
		if restoreCmd.NArg() < 1 {
			fmt.Println("Usage: dcsbr restore [--target DIR] <backup-archive>")
			os.Exit(1)
		}
		archivePath := restoreCmd.Arg(0)
		// Extract stack name from archive filename
		stackName := extractStackNameFromArchive(archivePath)
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
		if err := backup.RestoreFromBackup(archivePath, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Restore failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Restore completed successfully.")
		return
	}
	cfg, err := backup.LoadConfig("config.yaml")
	if err != nil {
		fmt.Println("Error loading config.yaml:", err)
		os.Exit(1)
	}
	for _, srcPath := range cfg.Backup.Sources {
		absSrc, _ := filepath.Abs(srcPath)
		absDst, _ := filepath.Abs(cfg.Backup.Target)
		fmt.Printf("Starting backup of '%s' to '%s' (formats: %v)...\n", absSrc, absDst, cfg.Backup.Formats)
		err := backup.BackupComposeStackWithFormats(absSrc, absDst, cfg.Backup.Formats)
		if err != nil {
			fmt.Printf("Error backing up %s: %v\n", absSrc, err)
		}
	}
	fmt.Println("All backups completed.")
}

func extractStackNameFromArchive(archivePath string) string {
	base := filepath.Base(archivePath)
	re := regexp.MustCompile(`^backup_(.+?)_\d{8}_\d{6}\.(tar\.gz|zip)$`)
	matches := re.FindStringSubmatch(base)
	if len(matches) > 1 {
		return matches[1]
	}
	// fallback: remove extension and backup_ prefix
	name := base
	if strings.HasPrefix(name, "backup_") {
		name = name[len("backup_"):]
	}
	if idx := strings.Index(name, "_"); idx > 0 {
		name = name[:idx]
	}
	return name
}
