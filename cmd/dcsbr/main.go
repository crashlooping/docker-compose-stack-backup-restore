package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/backup"
)

func main() {
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
