package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		SmsTCPPort    int    `yaml:"sms_tcp_port"`
		NotifyTCPPort int    `yaml:"notify_tcp_port"`
		WSPort        int    `yaml:"ws_port"`
		AuthToken     string `yaml:"auth_token"`
		LogFile       string `yaml:"log_file"`
		MaxBuffer     int    `yaml:"max_buffer"`
	} `yaml:"server"`
}

// ResolveLogPath returns the absolute log file path.
// Defaults to "relay-server.log" in the config file's directory.
func (c *Config) ResolveLogPath(cfgPath string) string {
	dir := filepath.Dir(cfgPath)
	if c.Server.LogFile != "" {
		if filepath.IsAbs(c.Server.LogFile) {
			return c.Server.LogFile
		}
		return filepath.Join(dir, c.Server.LogFile)
	}
	return filepath.Join(dir, "relay-server.log")
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	cfg.Server.SmsTCPPort = 1885
	cfg.Server.NotifyTCPPort = 1884
	cfg.Server.WSPort = 9091
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server.AuthToken == "" {
		return nil, fmt.Errorf("auth_token is required in config")
	}
	return cfg, nil
}
