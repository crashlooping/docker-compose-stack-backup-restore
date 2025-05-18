package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func BackupComposeStack(srcPath, dstPath string) error {
	composeFile, err := FindComposeFile(srcPath)
	if err != nil {
		return err
	}
	PrintComposeFileStatus(composeFile)
	stackWasRunning, err := StopStackIfRunning(srcPath, composeFile)
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
	err = TarGzFolderWithVolumes(srcPath, backupPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("tar.gz backup created.")

	zipName := fmt.Sprintf("backup_%s_%s.zip", folderName, timestamp)
	zipPath := filepath.Join(dstPath, zipName)
	fmt.Printf("Creating zip backup: %s\n", zipPath)
	err = ZipFolderWithVolumes(srcPath, zipPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("zip backup created.")

	for _, f := range volumeTarballs {
		os.Remove(f)
	}

	if stackWasRunning {
		fmt.Println("Restarting stack...")
		err = ComposeUp(srcPath, composeFile)
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
	volumes, err := ListComposeVolumes(srcPath, composeFile)
	if err != nil {
		return nil, err
	}
	stackName := filepath.Base(srcPath)
	for _, v := range volumes {
		fullVolumeName := stackName + "_" + v
		mountPath := GetVolumeMountPathFromCompose(composeFile, v, srcPath)
		tarPath, err := ExportDockerVolumeTar(fullVolumeName, mountPath)
		if err != nil {
			fmt.Printf("Warning: could not export volume %s: %v\n", fullVolumeName, err)
			continue
		}
		volumeTarballs = append(volumeTarballs, tarPath)
		fmt.Printf("Exported volume %s to %s\n", fullVolumeName, tarPath)
	}
	return volumeTarballs, nil
}
