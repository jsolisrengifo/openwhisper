package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// HotkeyConfig stores a global keyboard shortcut definition.
type HotkeyConfig struct {
	Modifiers uint32 `json:"modifiers"` // Win32 MOD_* flags (excl. MOD_NOREPEAT)
	VKey      uint32 `json:"vkey"`      // Win32 virtual-key code
	Display   string `json:"display"`   // Human-readable label, e.g. "Ctrl+Space"
}

// Settings holds the app configuration
type Settings struct {
	APIKey string       `json:"api_key"`
	Model  string       `json:"model"`
	Hotkey HotkeyConfig `json:"hotkey"`
}

// DefaultSettings returns the default configuration
func DefaultSettings() Settings {
	return Settings{
		Hotkey: HotkeyConfig{
			Modifiers: 0x0002, // MOD_CONTROL
			VKey:      0x20,   // VK_SPACE
			Display:   "Ctrl+Space",
		},
	}
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
