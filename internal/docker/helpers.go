package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const DockerComposeCmd = "docker compose"

func FindComposeFile(dir string) (string, error) {
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

func PrintComposeFileStatus(composeFile string) {
	if composeFile != "" {
		fmt.Printf("Found compose file: %s\n", composeFile)
	} else {
		fmt.Println("No docker-compose.yml or docker-compose.yaml found. Proceeding with backup only.")
	}
}

func StopStackIfRunning(srcPath, composeFile string) (bool, error) {
	if composeFile == "" {
		return false, nil
	}
	isRunning, err := IsComposeStackRunning(srcPath, composeFile)
	if err != nil {
		return false, err
	}
	if isRunning {
		fmt.Println("Stack is running. Stopping stack...")
		err = ComposeDown(srcPath, composeFile)
		if err != nil {
			return false, err
		}
		fmt.Println("Stack stopped.")
		return true, nil
	}
	fmt.Println("Stack is not running.")
	return false, nil
}

func IsComposeStackRunning(dir, composeFile string) (bool, error) {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "-q")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, nil // treat as not running if error
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func ComposeDown(dir, composeFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ComposeUp(dir, composeFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ListComposeVolumes(dir, composeFile string) ([]string, error) {
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

func GetVolumeMountPathFromCompose(composeFile, volume, srcPath string) string {
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
