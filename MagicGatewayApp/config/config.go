package config

import (
	"fmt"
	"os"
	"strings"

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

	// Environment variable overrides — secrets should never be in config files.
	if v := os.Getenv("MAGIC_API_KEY"); v != "" {
		cfg.DeepSeek.APIKey = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Server.JWTSecret = v
	}

	if cfg.Server.JWTSecret == "" || strings.Contains(cfg.Server.JWTSecret, "change-me-to-a-random") {
		return nil, fmt.Errorf("server.jwt_secret must be set (via config or JWT_SECRET env)")
	}
	if len(cfg.Server.JWTSecret) < 32 {
		return nil, fmt.Errorf("server.jwt_secret must be at least 32 characters (current: %d)", len(cfg.Server.JWTSecret))
	}

	if cfg.DeepSeek.APIKey == "" || cfg.DeepSeek.APIKey == "sk-your-deepseek-key" {
		return nil, fmt.Errorf("deepseek.api_key must be set (via config or MAGIC_API_KEY env)")
	}

	if cfg.DeepSeek.BaseURL == "" {
		cfg.DeepSeek.BaseURL = "https://api.deepseek.com/anthropic"
	}

	return cfg, nil
}
