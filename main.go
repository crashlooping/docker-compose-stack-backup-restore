package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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
	// Create backup file name
	folderName := filepath.Base(srcPath)
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("backup_%s_%s.tar.gz", folderName, timestamp)
	backupPath := filepath.Join(dstPath, backupName)
	fmt.Printf("Creating tar.gz backup: %s\n", backupPath)
	err = tarGzFolder(srcPath, backupPath)
	if err != nil {
		return err
	}
	fmt.Println("tar.gz backup created.")
	// Create zip
	zipName := fmt.Sprintf("backup_%s_%s.zip", folderName, timestamp)
	zipPath := filepath.Join(dstPath, zipName)
	fmt.Printf("Creating zip backup: %s\n", zipPath)
	err = zipFolder(srcPath, zipPath)
	if err != nil {
		return err
	}
	fmt.Println("zip backup created.")
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
