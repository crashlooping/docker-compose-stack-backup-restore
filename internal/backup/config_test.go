package backup

import (
	"os"
	"testing"
)

func TestLoadConfigValid(t *testing.T) {
	tmp := t.TempDir()
	file := tmp + "/config.yaml"
	os.WriteFile(file, []byte(`backup:
  formats: ["tar.gz", "zip"]
  sources:
    - foo
  target: bar
  prefix: dcsbr
`), 0o644)
	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if cfg.Backup.Target != "bar" {
		t.Errorf("Expected target 'bar', got %v", cfg.Backup.Target)
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
