package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ClientConfig struct {
	ServerHost string `json:"server_host"`
	WSPort     int    `json:"ws_port"`
	AuthToken  string `json:"auth_token"`
	WebPort    int    `json:"web_port"`
}

func DefaultConfig() ClientConfig {
	return ClientConfig{
		ServerHost: "127.0.0.1",
		WSPort:     9091,
		AuthToken:  "change-me-to-a-secret-token",
		WebPort:    19800,
	}
}

func configPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "config.json"), nil
}

func LoadClientConfig() (ClientConfig, error) {
	cfg := DefaultConfig()
	cp, err := configPath()
	if err != nil {
		return cfg, nil // use defaults
	}
	data, err := os.ReadFile(cp)
	if err != nil {
		if os.IsNotExist(err) {
			// Write default config on first run.
			if saveErr := SaveClientConfig(cfg); saveErr != nil {
				return cfg, nil
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func SaveClientConfig(cfg ClientConfig) error {
	cp, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cp, data, 0644)
}
