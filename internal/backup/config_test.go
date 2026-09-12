package backup

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfigValid(t *testing.T) {
	tmp := t.TempDir()
	srcDir := tmp + "/mysource"
	tgtDir := tmp + "/backup"
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tgtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := tmp + "/config.yaml"
	os.WriteFile(file, []byte("backup:\n  formats: [\"tar.gz\", \"zip\"]\n  sources:\n    - "+srcDir+"\n  target: "+tgtDir+"\n  prefix: dcsbr\n"), 0o644)
	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if cfg.Backup.Target != tgtDir {
		t.Errorf("Expected target '%s', got %v", tgtDir, cfg.Backup.Target)
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

func TestLoadConfigSourcesFirstLast(t *testing.T) {
	tmp := t.TempDir()
	first := tmp + "/first"
	regular := tmp + "/regular"
	tgtDir := tmp + "/backup"
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(regular, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tgtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := tmp + "/config.yaml"
	os.WriteFile(file, []byte("backup:\n  formats: [\"tar.gz\"]\n  sources:\n    - "+regular+"\n  sources_first_last:\n    - "+first+"\n  target: "+tgtDir+"\n  prefix: dcsbr\n"), 0o644)
	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(cfg.Backup.SourcesFirstLast) != 1 || cfg.Backup.SourcesFirstLast[0] != first {
		t.Errorf("Expected sources_first_last to contain %s, got %v", first, cfg.Backup.SourcesFirstLast)
	}
}

func TestLoadConfigSourcesFirstLastMultiple(t *testing.T) {
	tmp := t.TempDir()
	first := tmp + "/first"
	second := tmp + "/second"
	regular := tmp + "/regular"
	tgtDir := tmp + "/backup"
	for _, d := range []string{first, second, regular, tgtDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file := tmp + "/config.yaml"
	os.WriteFile(file, []byte("backup:\n  formats: [\"tar.gz\"]\n  sources:\n    - "+regular+"\n  sources_first_last:\n    - "+first+"\n    - "+second+"\n  target: "+tgtDir+"\n  prefix: dcsbr\n"), 0o644)
	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(cfg.Backup.SourcesFirstLast) != 2 {
		t.Errorf("Expected 2 sources_first_last entries, got %d", len(cfg.Backup.SourcesFirstLast))
	}
	if cfg.Backup.SourcesFirstLast[0] != first || cfg.Backup.SourcesFirstLast[1] != second {
		t.Errorf("Expected sources_first_last to be [%s, %s], got %v", first, second, cfg.Backup.SourcesFirstLast)
	}
}

func TestLoadConfigSourcesFirstLastDuplicate(t *testing.T) {
	tmp := t.TempDir()
	src := tmp + "/dup"
	tgtDir := tmp + "/backup"
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tgtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := tmp + "/config.yaml"
	os.WriteFile(file, []byte("backup:\n  formats: [\"tar.gz\"]\n  sources:\n    - "+src+"\n  sources_first_last:\n    - "+src+"\n  target: "+tgtDir+"\n  prefix: dcsbr\n"), 0o644)
	_, err := LoadConfig(file)
	if err == nil {
		t.Fatal("Expected error for duplicate source in sources and sources_first_last")
	}
	if !strings.Contains(err.Error(), "both 'sources' and 'sources_first_last'") {
		t.Fatalf("Expected duplicate-source error, got: %v", err)
	}
}
