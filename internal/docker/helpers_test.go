package docker

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testComposeYml  = "docker-compose.yml"
	testComposeYaml = "docker-compose.yaml"
	nonexistentDir  = "/nonexistent"
)

func TestDockerHelpersSanity(t *testing.T) {
	// This is a placeholder test. Add real tests for docker helpers here.
}

func TestFindComposeFileYmlAndYaml(t *testing.T) {
	dir := t.TempDir()
	composeYmlPath := filepath.Join(dir, testComposeYml)
	composeYamlPath := filepath.Join(dir, testComposeYaml)
	os.WriteFile(composeYmlPath, []byte("version: '3'"), 0o644)
	file, err := FindComposeFile(dir)
	if err != nil || file != testComposeYml {
		t.Errorf("Expected %s, got %v, err: %v", testComposeYml, file, err)
	}
	os.Remove(composeYmlPath)
	os.WriteFile(composeYamlPath, []byte("version: '3'"), 0o644)
	file, err = FindComposeFile(dir)
	if err != nil || file != testComposeYaml {
		t.Errorf("Expected %s, got %v, err: %v", testComposeYaml, file, err)
	}
	os.Remove(composeYamlPath)
	file, err = FindComposeFile(dir)
	if err != nil || file != "" {
		t.Errorf("Expected '', got %v, err: %v", file, err)
	}
}

func TestGetVolumeMountPathFromCompose(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, testComposeYml)
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
	mountPath := GetVolumeMountPathFromCompose(testComposeYml, "myvol", dir)
	if mountPath != "/data" {
		t.Errorf("Expected /data, got %v", mountPath)
	}
}

func TestPrintComposeFileStatus(t *testing.T) {
	PrintComposeFileStatus(testComposeYml)
	PrintComposeFileStatus("")
}

func TestStopStackIfRunningEmptyComposeFile(t *testing.T) {
	stopped, err := StopStackIfRunning(nonexistentDir, "")
	if stopped || err != nil {
		t.Errorf("Expected false, nil for empty composeFile, got %v, %v", stopped, err)
	}
}

func TestIsComposeStackRunningError(t *testing.T) {
	_, err := IsComposeStackRunning(nonexistentDir, testComposeYml)
	if err != nil {
		t.Errorf("Expected nil error (treated as not running), got %v", err)
	}
}

func TestComposeDownUpError(t *testing.T) {
	err := ComposeDown(nonexistentDir, testComposeYml)
	if err == nil {
		t.Error("Expected error for ComposeDown on nonexistent dir")
	}
	err = ComposeUp(nonexistentDir, testComposeYml)
	if err == nil {
		t.Error("Expected error for ComposeUp on nonexistent dir")
	}
}

func TestListComposeVolumesError(t *testing.T) {
	_, err := ListComposeVolumes(nonexistentDir, testComposeYml)
	if err == nil {
		t.Error("Expected error for ListComposeVolumes on nonexistent dir")
	}
}

func TestGetVolumeMountPathFromComposeFallback(t *testing.T) {
	mountPath := GetVolumeMountPathFromCompose("doesnotexist.yml", "vol", t.TempDir())
	if mountPath != "/volume" {
		t.Errorf("Expected fallback /volume, got %v", mountPath)
	}
}
