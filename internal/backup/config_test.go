package backup

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfigValid(t *testing.T) {
	tmp := t.TempDir()
	srcDir := tmp + "/mysource"
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := tmp + "/config.yaml"
	os.WriteFile(file, []byte("backup:\n  formats: [\"tar.gz\", \"zip\"]\n  sources:\n    - "+srcDir+"\n  target: "+tmp+"/backup\n  prefix: dcsbr\n"), 0o644)
	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if cfg.Backup.Target != tmp+"/backup" {
		t.Errorf("Expected target '%s', got %v", tmp+"/backup", cfg.Backup.Target)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	file := tmp + "/bad.yaml"
	os.WriteFile(file, []byte(`not: yaml: [}`), 0o644)
	_, err := LoadConfig(file)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfigSourceNotFound(t *testing.T) {
	tmp := t.TempDir()
	file := tmp + "/config.yaml"
	os.WriteFile(file, []byte("backup:\n  formats: [\"tar.gz\"]\n  sources:\n    - "+tmp+"/nonexistent\n  target: "+tmp+"/backup\n  prefix: dcsbr\n"), 0o644)
	_, err := LoadConfig(file)
	if err == nil {
		t.Fatal("Expected error for non-existent source path")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Expected 'does not exist' error, got: %v", err)
	}
}
