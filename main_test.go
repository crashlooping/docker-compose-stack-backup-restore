package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindComposeFile(t *testing.T) {
	dir := t.TempDir()
	composeYml := filepath.Join(dir, "docker-compose.yml")
	composeYaml := filepath.Join(dir, "docker-compose.yaml")
	os.WriteFile(composeYml, []byte("version: '3'"), 0o644)
	file, err := findComposeFile(dir)
	if err != nil || file != "docker-compose.yml" {
		t.Errorf("Expected docker-compose.yml, got %v, err: %v", file, err)
	}
	os.Remove(composeYml)
	os.WriteFile(composeYaml, []byte("version: '3'"), 0o644)
	file, err = findComposeFile(dir)
	if err != nil || file != "docker-compose.yaml" {
		t.Errorf("Expected docker-compose.yaml, got %v, err: %v", file, err)
	}
	os.Remove(composeYaml)
	file, err = findComposeFile(dir)
	if err != nil || file != "" {
		t.Errorf("Expected '', got %v, err: %v", file, err)
	}
}

func TestTarGzFolderAndZipFolder(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello world"), 0o644)
	os.Mkdir(filepath.Join(src, ".git"), 0o755)
	os.WriteFile(filepath.Join(src, ".git", "should_ignore.txt"), []byte("ignore me"), 0o644)
	dst := t.TempDir()
	tarPath := filepath.Join(dst, "test.tar.gz")
	zipPath := filepath.Join(dst, "test.zip")
	if err := tarGzFolder(src, tarPath); err != nil {
		t.Fatalf("tarGzFolder failed: %v", err)
	}
	if err := zipFolder(src, zipPath); err != nil {
		t.Fatalf("zipFolder failed: %v", err)
	}
	// Optionally, check that .git/should_ignore.txt is not in the archives (not implemented here)
}
