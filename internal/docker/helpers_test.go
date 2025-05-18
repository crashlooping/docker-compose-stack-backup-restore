package docker

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	composeYml  = "docker-compose.yml"
	composeYaml = "docker-compose.yaml"
)

func TestDockerHelpersSanity(t *testing.T) {
	// This is a placeholder test. Add real tests for docker helpers here.
}

func TestFindComposeFileYmlAndYaml(t *testing.T) {
	dir := t.TempDir()
	composeYmlPath := filepath.Join(dir, composeYml)
	composeYamlPath := filepath.Join(dir, composeYaml)
	os.WriteFile(composeYmlPath, []byte("version: '3'"), 0o644)
	file, err := FindComposeFile(dir)
	if err != nil || file != composeYml {
		t.Errorf("Expected %s, got %v, err: %v", composeYml, file, err)
	}
	os.Remove(composeYmlPath)
	os.WriteFile(composeYamlPath, []byte("version: '3'"), 0o644)
	file, err = FindComposeFile(dir)
	if err != nil || file != composeYaml {
		t.Errorf("Expected %s, got %v, err: %v", composeYaml, file, err)
	}
	os.Remove(composeYamlPath)
	file, err = FindComposeFile(dir)
	if err != nil || file != "" {
		t.Errorf("Expected '', got %v, err: %v", file, err)
	}
}

func TestGetVolumeMountPathFromCompose(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, composeYml)
	content := `
version: '3'
services:
  app:
    image: busybox
    volumes:
      - myvol:/data
volumes:
  myvol:
`
	os.WriteFile(composeFile, []byte(content), 0o644)
	mountPath := GetVolumeMountPathFromCompose(composeYml, "myvol", dir)
	if mountPath != "/data" {
		t.Errorf("Expected /data, got %v", mountPath)
	}
}
