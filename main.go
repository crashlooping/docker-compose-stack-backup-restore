package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const dockerComposeCmd = "docker compose"

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: dcsbr <source_folder> <destination_folder>")
		fmt.Println("Example: dcsbr C:/my/stack C:/my/backups")
		os.Exit(1)
	}
	srcPath := os.Args[1]
	dstPath := os.Args[2]
	fmt.Printf("Starting backup of '%s' to '%s'...\n", srcPath, dstPath)
	err := BackupComposeStack(srcPath, dstPath)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("Backup completed successfully.")
}

// BackupComposeStack backs up a docker-compose stack folder as described.
func BackupComposeStack(srcPath, dstPath string) error {
	composeFile, err := findComposeFile(srcPath)
	if err != nil {
		return err
	}
	printComposeFileStatus(composeFile)
	stackWasRunning, err := stopStackIfRunning(srcPath, composeFile)
	if err != nil {
		return err
	}

	volumeTarballs, err := exportAllComposeVolumes(srcPath, composeFile)
	if err != nil {
		return err
	}

	// Step 2: Create backup file name
	folderName := filepath.Base(srcPath)
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("backup_%s_%s.tar.gz", folderName, timestamp)
	backupPath := filepath.Join(dstPath, backupName)
	fmt.Printf("Creating tar.gz backup: %s\n", backupPath)
	err = tarGzFolderWithVolumes(srcPath, backupPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("tar.gz backup created.")

	zipName := fmt.Sprintf("backup_%s_%s.zip", folderName, timestamp)
	zipPath := filepath.Join(dstPath, zipName)
	fmt.Printf("Creating zip backup: %s\n", zipPath)
	err = zipFolderWithVolumes(srcPath, zipPath, volumeTarballs)
	if err != nil {
		return err
	}
	fmt.Println("zip backup created.")

	// Clean up temp volume tarballs
	for _, f := range volumeTarballs {
		os.Remove(f)
	}

	if stackWasRunning {
		fmt.Println("Restarting stack...")
		err = composeUp(srcPath, composeFile)
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
	volumes, err := listComposeVolumes(srcPath, composeFile)
	if err != nil {
		return nil, err
	}
	stackName := filepath.Base(srcPath)
	for _, v := range volumes {
		fullVolumeName := stackName + "_" + v
		mountPath := getVolumeMountPathFromCompose(composeFile, v, srcPath)
		tarPath, err := exportDockerVolumeTar(fullVolumeName, mountPath)
		if err != nil {
			fmt.Printf("Warning: could not export volume %s: %v\n", fullVolumeName, err)
			continue
		}
		volumeTarballs = append(volumeTarballs, tarPath)
		fmt.Printf("Exported volume %s to %s\n", fullVolumeName, tarPath)
	}
	return volumeTarballs, nil
}

func printComposeFileStatus(composeFile string) {
	if composeFile != "" {
		fmt.Printf("Found compose file: %s\n", composeFile)
	} else {
		fmt.Println("No docker-compose.yml or docker-compose.yaml found. Proceeding with backup only.")
	}
}

func stopStackIfRunning(srcPath, composeFile string) (bool, error) {
	if composeFile == "" {
		return false, nil
	}
	isRunning, err := isComposeStackRunning(srcPath, composeFile)
	if err != nil {
		return false, err
	}
	if isRunning {
		fmt.Println("Stack is running. Stopping stack...")
		err = composeDown(srcPath, composeFile)
		if err != nil {
			return false, err
		}
		fmt.Println("Stack stopped.")
		return true, nil
	}
	fmt.Println("Stack is not running.")
	return false, nil
}

func findComposeFile(dir string) (string, error) {
	yml := filepath.Join(dir, "docker-compose.yml")
	yaml := filepath.Join(dir, "docker-compose.yaml")
	if _, err := os.Stat(yml); err == nil {
		return "docker-compose.yml", nil
	}
	if _, err := os.Stat(yaml); err == nil {
		return "docker-compose.yaml", nil
	}
	return "", nil // not an error if not found
}

func isComposeStackRunning(dir, composeFile string) (bool, error) {
	cmd := exec.Command("cmd", "/C", dockerComposeCmd, "-f", composeFile, "ps", "-q")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, nil // treat as not running if error
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func composeDown(dir, composeFile string) error {
	cmd := exec.Command("cmd", "/C", dockerComposeCmd, "-f", composeFile, "down")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func composeUp(dir, composeFile string) error {
	cmd := exec.Command("cmd", "/C", dockerComposeCmd, "-f", composeFile, "up", "-d")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func tarGzFolder(srcDir, destFile string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tarw := tar.NewWriter(gz)
	defer tarw.Close()
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		relPath, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		// Ignore .git folder and its contents
		if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return addFileToTar(srcDir, path, info, err, tarw)
	})
}

func addFileToTar(srcDir, path string, info os.FileInfo, err error, tarw *tar.Writer) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	if relPath == "." {
		return nil
	}
	hdr, err := tar.FileInfoHeader(info, path)
	if err != nil {
		return err
	}
	hdr.Name = relPath
	if err := tarw.WriteHeader(hdr); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tarw, file)
		if err != nil {
			return err
		}
	}
	return nil
}

func zipFolder(srcDir, destFile string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	zipw := zip.NewWriter(f)
	defer zipw.Close()
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		relPath, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		// Ignore .git folder and its contents
		if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		w, err := zipw.Create(relPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, file)
		return err
	})
}

// listComposeVolumes parses docker compose config to get all named volumes
func listComposeVolumes(dir, composeFile string) ([]string, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "config", "--format", "json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var config struct {
		Volumes map[string]interface{} `json:"volumes"`
	}
	if err := json.Unmarshal(out, &config); err != nil {
		return nil, err
	}
	var names []string
	for k := range config.Volumes {
		names = append(names, k)
	}
	return names, nil
}

// exportDockerVolumeTar uses a sidecar container to tar a volume to a temp file
// Now accepts a mountPath argument for the correct mount point inside the container
func exportDockerVolumeTar(volume, mountPath string) (string, error) {
	tarPath := filepath.Join(os.TempDir(), volume+".tar")
	containerName := "tmp-vol-backup-" + volume + "-" + fmt.Sprint(time.Now().UnixNano())
	fmt.Printf("Exporting docker volume '%s' (mount path: %s) to tarball. Host path: %s\n", volume, mountPath, tarPath)
	// Remove verbose file listing
	// Now tar the volume
	tarCmd := fmt.Sprintf("tar cf /backup/%s.tar -C %s .", volume, mountPath)
	cmd := exec.Command("docker", "run", "--rm", "--name", containerName, "-v", volume+":"+mountPath+":ro", "-v", os.TempDir()+":/backup", "alpine", "sh", "-c", tarCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return tarPath, nil
}

func tarGzFolderWithVolumes(srcDir, destFile string, volumeTarballs []string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tarw := tar.NewWriter(gz)
	defer tarw.Close()
	// Add filesystem
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		return addFileToTarWithGitIgnore(srcDir, path, info, err, tarw)
	})
	if err != nil {
		return err
	}
	// Add volumes
	for _, v := range volumeTarballs {
		if err := addVolumeTarToTarGz(v, tarw); err != nil {
			return err
		}
	}
	return nil
}

func addFileToTarWithGitIgnore(srcDir, path string, info os.FileInfo, err error, tarw *tar.Writer) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	return addFileToTar(srcDir, path, info, err, tarw)
}

func addVolumeTarToTarGz(volumeTarPath string, tarw *tar.Writer) error {
	file, err := os.Open(volumeTarPath)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fmt.Printf("Adding volume tarball '%s' to backup (size: %d bytes)\n", volumeTarPath, stat.Size())
	hdr := &tar.Header{
		Name:    filepath.Join("volumes", filepath.Base(volumeTarPath)),
		Size:    stat.Size(),
		Mode:    0o600,
		ModTime: stat.ModTime(),
	}
	if err := tarw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tarw, file)
	return err
}

func zipFolderWithVolumes(srcDir, destFile string, volumeTarballs []string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	zipw := zip.NewWriter(f)
	defer zipw.Close()
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		return addFileToZipWithGitIgnore(srcDir, path, info, err, zipw)
	})
	if err != nil {
		return err
	}
	for _, v := range volumeTarballs {
		if err := addVolumeTarToZip(v, zipw); err != nil {
			return err
		}
	}
	return nil
}

func addFileToZipWithGitIgnore(srcDir, path string, info os.FileInfo, err error, zipw *zip.Writer) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if info.IsDir() {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w, err := zipw.Create(relPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, file)
	return err
}

func addVolumeTarToZip(volumeTarPath string, zipw *zip.Writer) error {
	file, err := os.Open(volumeTarPath)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fmt.Printf("Adding volume tarball '%s' to zip backup (size: %d bytes)\n", volumeTarPath, stat.Size())
	w, err := zipw.Create(filepath.Join("volumes", filepath.Base(volumeTarPath)))
	if err != nil {
		return err
	}
	_, err = io.Copy(w, file)
	return err
}

// getVolumeMountPathFromCompose parses the compose file to find the mount path for a given volume
func getVolumeMountPathFromCompose(composeFile, volume, srcPath string) string {
	composePath := filepath.Join(srcPath, composeFile)
	f, err := os.Open(composePath)
	if err != nil {
		return "/volume" // fallback
	}
	defer f.Close()
	var mountPath string = "/volume" // default fallback
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, volume+":") {
			// Try to extract the mount path after the colon
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				candidate := strings.TrimSpace(parts[1])
				if !strings.HasPrefix(candidate, "/") {
					continue // skip relative paths
				}
				mountPath = candidate
				break
			}
		}
	}
	return mountPath
}
