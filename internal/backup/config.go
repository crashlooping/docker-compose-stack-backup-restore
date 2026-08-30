package backup

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Backup struct {
		Formats      []string `yaml:"formats"`
		Sources      []string `yaml:"sources"`
		Target       string   `yaml:"target"`
		Password     string   `yaml:"password"`
		MaxBackups   int      `yaml:"max_backups"`
		Prefix       string   `yaml:"prefix"`
		SudoRequired bool     `yaml:"sudo_required"`
	} `yaml:"backup"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (cfg *Config) validate() error {
	if cfg.Backup.Prefix == "" {
		return fmt.Errorf("'prefix' is required in config.yaml under 'backup'")
	}
	if len(cfg.Backup.Formats) == 0 {
		return fmt.Errorf("at least one format is required in 'formats' (supported: tar.gz, zip)")
	}
	for _, f := range cfg.Backup.Formats {
		if f != "tar.gz" && f != "zip" {
			return fmt.Errorf("unsupported format '%s' in config (supported: tar.gz, zip)", f)
		}
	}
	if cfg.Backup.Target == "" {
		return fmt.Errorf("'target' is required in config.yaml under 'backup'")
	}
	// Verify target directory exists and is writable
	targetDir := cfg.Backup.Target
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("target directory '%s' cannot be created: %w", targetDir, err)
	}
	tmpFile, err := os.CreateTemp(targetDir, ".dcsbr-write-test-*")
	if err != nil {
		return fmt.Errorf("target directory '%s' is not writable: %w", targetDir, err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())
	if cfg.Backup.Password != "" && len(cfg.Backup.Password) < 16 {
		return fmt.Errorf("'password' must be at least 16 characters long")
	}
	// maxBackups: 0 means unlimited (no pruning), negative is invalid
	if cfg.Backup.MaxBackups < 0 {
		cfg.Backup.MaxBackups = 0
	}
	// Verify all source paths exist before starting any backup
	for i, src := range cfg.Backup.Sources {
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("source path '%s' (index %d) does not exist", src, i)
			}
			return fmt.Errorf("source path '%s' (index %d) is not accessible: %w", src, i, err)
		}
	}
	return nil
}
