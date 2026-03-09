package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringSvc  = "openwhisper"
	keyringUser = "gemini_api_key"
)

// HotkeyConfig stores a global keyboard shortcut definition.
type HotkeyConfig struct {
	Modifiers uint32 `json:"modifiers"` // Win32 MOD_* flags (excl. MOD_NOREPEAT)
	VKey      uint32 `json:"vkey"`      // Win32 virtual-key code
	Display   string `json:"display"`   // Human-readable label, e.g. "Ctrl+Space"
}

// Settings holds the app configuration.
// APIKey is NOT persisted in the JSON file; it lives in the OS keyring.
type Settings struct {
	APIKey  string       `json:"api_key,omitempty"` // Only in memory / transmitted to UI; never written to disk
	Model   string       `json:"model"`
	Hotkey  HotkeyConfig `json:"hotkey"`
	Opacity int          `json:"opacity"` // Widget window opacity: 10–100 (percent)
}

// diskSettings is the representation written to config.json.
// The API key is intentionally absent.
type diskSettings struct {
	Model   string       `json:"model"`
	Hotkey  HotkeyConfig `json:"hotkey"`
	Opacity int          `json:"opacity"`
}

// DefaultSettings returns the default configuration
func DefaultSettings() Settings {
	return Settings{
		Hotkey: HotkeyConfig{
			Modifiers: 0x0002, // MOD_CONTROL
			VKey:      0x20,   // VK_SPACE
			Display:   "Ctrl+Space",
		},
		Opacity: 100,
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

// saveAPIKey stores the API key in the OS keyring.
// If key is empty the existing entry is deleted (best-effort).
func saveAPIKey(key string) error {
	if key == "" {
		err := keyring.Delete(keyringSvc, keyringUser)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
		return nil
	}
	return keyring.Set(keyringSvc, keyringUser, key)
}

// loadAPIKey retrieves the API key from the OS keyring.
// Returns ("", nil) when no key has been stored yet.
func loadAPIKey() (string, error) {
	val, err := keyring.Get(keyringSvc, keyringUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// LoadSettings reads settings from disk and loads the API key from the OS keyring.
// Returns DefaultSettings when the config file does not yet exist (first run).
func LoadSettings() (*Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			def := DefaultSettings()
			return &def, nil
		}
		return nil, err
	}

	var d diskSettings
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}

	s := Settings{
		Model:   d.Model,
		Hotkey:  d.Hotkey,
		Opacity: d.Opacity,
	}
	if s.Opacity == 0 {
		s.Opacity = 100
	}

	// Load the API key from the secure keyring (best-effort: empty on error)
	s.APIKey, _ = loadAPIKey()

	return &s, nil
}

// saveSettings persists non-sensitive settings to disk and stores the API key
// in the OS native keyring (Windows Credential Manager / macOS Keychain / Linux Secret Service).
func saveSettings(s Settings) error {
	// 1. Persist the API key securely
	if err := saveAPIKey(s.APIKey); err != nil {
		return err
	}

	// 2. Write non-sensitive config to disk (APIKey deliberately excluded)
	path, err := settingsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	d := diskSettings{
		Model:   s.Model,
		Hotkey:  s.Hotkey,
		Opacity: s.Opacity,
	}

	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	// Write with restrictive permissions (only owner can read/write)
	return os.WriteFile(path, data, 0600)
}
