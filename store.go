package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds the app configuration
type Settings struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

// DefaultSettings returns the default configuration
func DefaultSettings() Settings {
	return Settings{}
}

// settingsPath returns the path to the config file
func settingsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "openwhisper", "config.json"), nil
}

// LoadSettings reads settings from disk. Returns DefaultSettings on any error.
func LoadSettings() (*Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

// saveSettings writes settings to disk
func saveSettings(s Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Write with restrictive permissions (only owner can read/write)
	return os.WriteFile(path, data, 0600)
}
