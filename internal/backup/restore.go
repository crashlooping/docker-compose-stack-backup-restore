package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/archive"
	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/docker"
)

type RestoreOptions struct {
	TargetDir string // where to extract stack folder (and volumes if not docker)
}

// RestoreFromBackup extracts the stack and volumes from a backup archive.
func RestoreFromBackup(archivePath string, opts RestoreOptions) error {
	fmt.Printf("[restore] Creating target folder: %s\n", opts.TargetDir)
	if err := os.MkdirAll(opts.TargetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target folder: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "dcsbr-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("[restore] Extracting archive %s to temp dir...\n", archivePath)
	if strings.HasSuffix(archivePath, ".tar.gz") {
		err = archive.ExtractTarGz(archivePath, tmpDir)
	} else if strings.HasSuffix(archivePath, ".zip") {
		err = archive.ExtractZip(archivePath, tmpDir)
	} else {
		return fmt.Errorf("unsupported archive format: %s", archivePath)
	}
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	fmt.Printf("[restore] Copying stack folders to %s...\n", opts.TargetDir)
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if entry.Name() == "volumes" {
			continue
		}
		src := filepath.Join(tmpDir, entry.Name())
		dst := filepath.Join(opts.TargetDir, entry.Name())
		fmt.Printf("[restore]   Copying %s -> %s\n", src, dst)
		if err := archive.CopyDir(src, dst); err != nil {
			return fmt.Errorf("failed to copy stack folder: %w", err)
		}
	}

	// 3. Restore volumes if present (docker volumes)
	volDir := filepath.Join(tmpDir, "volumes")
	if stat, err := os.Stat(volDir); err == nil && stat.IsDir() {
		fmt.Printf("[restore] Restoring docker volumes from %s...\n", volDir)
		volEntries, _ := os.ReadDir(volDir)
		for _, v := range volEntries {
			volTar := filepath.Join(volDir, v.Name())
			volName := strings.TrimSuffix(v.Name(), ".tar")
			fmt.Printf("[restore]   Restoring volume %s from %s\n", volName, volTar)
			if err := docker.RestoreVolumeFromTar(volName, volTar); err != nil {
				return fmt.Errorf("failed to restore docker volume %s: %w", volName, err)
			}
		}
	}
	fmt.Println("[restore] Restore process complete.")
	return nil
}
