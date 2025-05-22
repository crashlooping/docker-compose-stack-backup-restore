package backup

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Backup struct {
		Formats    []string `yaml:"formats"`
		Sources    []string `yaml:"sources"`
		Target     string   `yaml:"target"`
		Password   string   `yaml:"password"`
		MaxBackups int      `yaml:"max_backups"`
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
	if cfg.Backup.MaxBackups <= 0 {
		cfg.Backup.MaxBackups = 10
	}
	return &cfg, nil
}
