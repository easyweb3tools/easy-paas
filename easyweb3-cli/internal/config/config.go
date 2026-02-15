package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	APIBase  string `json:"api_base"`
	Project  string `json:"project"`
	LogLevel string `json:"log_level"`
}

type Credentials struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	APIKey    string `json:"api_key"`
}

func DefaultConfig() Config {
	return Config{
		APIBase:  "http://localhost:8080",
		Project:  "",
		LogLevel: "info",
	}
}

func Dir() (string, error) {
	// Allow sandboxes (e.g. PicoClaw workspace) to pin state to a writable/allowed directory.
	// This directory holds config.json and credentials.json.
	if v := strings.TrimSpace(os.Getenv("EASYWEB3_DIR")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("EASYWEB3_HOME")); v != "" {
		return filepath.Join(v, ".easyweb3"), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".easyweb3"), nil
}

func ConfigPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func CredentialsPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "credentials.json"), nil
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()

	p, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	if b, err := os.ReadFile(p); err == nil {
		var onDisk Config
		if err := json.Unmarshal(b, &onDisk); err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", p, err)
		}
		if strings.TrimSpace(onDisk.APIBase) != "" {
			cfg.APIBase = strings.TrimRight(strings.TrimSpace(onDisk.APIBase), "/")
		}
		if strings.TrimSpace(onDisk.Project) != "" {
			cfg.Project = strings.TrimSpace(onDisk.Project)
		}
		if strings.TrimSpace(onDisk.LogLevel) != "" {
			cfg.LogLevel = strings.TrimSpace(onDisk.LogLevel)
		}
	}

	// Env overrides
	if v := strings.TrimSpace(os.Getenv("EASYWEB3_API_BASE")); v != "" {
		cfg.APIBase = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("EASYWEB3_PROJECT")); v != "" {
		cfg.Project = v
	}

	if cfg.APIBase == "" {
		return Config{}, errors.New("api_base is empty")
	}
	return cfg, nil
}

func LoadCredentials() (Credentials, error) {
	p, err := CredentialsPath()
	if err != nil {
		return Credentials{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal(b, &c); err != nil {
		return Credentials{}, fmt.Errorf("parse %s: %w", p, err)
	}
	return c, nil
}

func SaveCredentials(c Credentials) error {
	d, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	p := filepath.Join(d, "credentials.json")
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

func (c Credentials) ExpiresAtTime() (time.Time, bool) {
	v := strings.TrimSpace(c.ExpiresAt)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
