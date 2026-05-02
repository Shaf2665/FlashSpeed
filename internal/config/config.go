package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	TLS     TLSConfig     `yaml:"tls"`
	Storage StorageConfig `yaml:"storage"`
	Admin   AdminConfig   `yaml:"admin"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	DataDir string `yaml:"data_dir"`
}

type TLSConfig struct {
	Mode     string `yaml:"mode"`
	Domain   string `yaml:"domain"`
	Email    string `yaml:"email"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type StorageConfig struct {
	AutoDetectDrives bool     `yaml:"auto_detect_drives"`
	ManualPaths      []string `yaml:"manual_paths"`
}

type AdminConfig struct {
	CreateDefaultAdmin bool `yaml:"create_default_admin"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Server.DataDir = "/var/lib/flashyspeed"
	cfg.TLS.Mode = "self-signed"
	cfg.Storage.AutoDetectDrives = true
	cfg.Admin.CreateDefaultAdmin = true

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	if v := os.Getenv("FS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("FS_DATA_DIR"); v != "" {
		cfg.Server.DataDir = v
	}

	return cfg, nil
}
