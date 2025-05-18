package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/archive"
	"github.com/crashlooping/docker-compose-stack-backup-restore/internal/docker"
)

func BackupComposeStack(srcPath, dstPath string) error {
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
	backupName := fmt.Sprintf("backup_%s_%s.tar.gz", folderName, timestamp)
	backupPath := filepath.Join(dstPath, backupName)
	fmt.Printf("Creating tar.gz backup: %s\n", backupPath)
	err = archive.TarGzFolderWithVolumes(srcPath, backupPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("tar.gz backup created.")

	zipName := fmt.Sprintf("backup_%s_%s.zip", folderName, timestamp)
	zipPath := filepath.Join(dstPath, zipName)
	fmt.Printf("Creating zip backup: %s\n", zipPath)
	err = archive.ZipFolderWithVolumes(srcPath, zipPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("zip backup created.")

	for _, f := range volumeTarballs {
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

func exportAllComposeVolumes(srcPath, composeFile string) ([]string, error) {
	var volumeTarballs []string
	if composeFile == "" {
		return volumeTarballs, nil
	}
	fmt.Println("Detecting and exporting docker volumes...")
	volumes, err := docker.ListComposeVolumes(srcPath, composeFile)
	if err != nil {
		return nil, err
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
		volumeTarballs = append(volumeTarballs, tarPath)
		fmt.Printf("Exported volume %s to %s\n", fullVolumeName, tarPath)
	}
	return volumeTarballs, nil
}
