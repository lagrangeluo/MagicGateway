package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	DeepSeek  DeepSeekConfig  `yaml:"deepseek"`
	Database  DatabaseConfig  `yaml:"database"`
	Admin     AdminConfig     `yaml:"admin"`
}

type ServerConfig struct {
	Port      int    `yaml:"port"`
	JWTSecret string `yaml:"jwt_secret"`
	LogFile   string `yaml:"log_file"`
}

type DeepSeekConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AdminConfig struct {
	DefaultUsername string `yaml:"default_username"`
	DefaultPassword string `yaml:"default_password"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:      8080,
			JWTSecret: "",
		},
		Database: DatabaseConfig{
			Path: "./data/magicgateway.db",
		},
		Admin: AdminConfig{
			DefaultUsername: "admin",
			DefaultPassword: "magic2026",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Server.JWTSecret == "" || cfg.Server.JWTSecret == "change-me-to-a-random-string" {
		return nil, fmt.Errorf("server.jwt_secret must be set to a random string (not the default)")
	}
	if len(cfg.Server.JWTSecret) < 32 {
		return nil, fmt.Errorf("server.jwt_secret must be at least 32 characters (current: %d)", len(cfg.Server.JWTSecret))
	}

	if cfg.DeepSeek.APIKey == "" || cfg.DeepSeek.APIKey == "sk-your-enterprise-key" {
		return nil, fmt.Errorf("deepseek.api_key must be set to your enterprise key")
	}

	if cfg.DeepSeek.BaseURL == "" {
		cfg.DeepSeek.BaseURL = "https://api.deepseek.com/anthropic"
	}

	return cfg, nil
}
