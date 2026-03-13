package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringSvc = "openwhisper"
)

// keyringUserForProvider returns the keyring username for the given provider.
// "gemini" maps to the legacy key so existing stored keys still work.
func keyringUserForProvider(provider string) string {
	switch provider {
	case "openrouter":
		return "openrouter_api_key"
	case "google_tts":
		return "google_tts_api_key"
	default:
		return "gemini_api_key"
	}
}

// HotkeyConfig stores a global keyboard shortcut definition.
type HotkeyConfig struct {
	Modifiers uint32 `json:"modifiers"` // Win32 MOD_* flags (excl. MOD_NOREPEAT)
	VKey      uint32 `json:"vkey"`      // Win32 virtual-key code
	Display   string `json:"display"`   // Human-readable label, e.g. "Ctrl+Space"
}

// DictationProfile defines a named transcription mode with a custom prompt.
type DictationProfile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// Settings holds the app configuration.
// APIKey and TTSAPIKey are NOT persisted in the JSON file; they live in the OS keyring.
type Settings struct {
	APIKey           string              `json:"api_key,omitempty"`            // Only in memory / transmitted to UI; never written to disk
	Model            string              `json:"model,omitempty"`              // Derived from Models[0]; kept for migration compat only
	Models           []string            `json:"models"`                       // Ordered list of models to try (fallback left-to-right)
	Provider         string              `json:"provider"`                     // "gemini" or "openrouter"
	ModelByProvider  map[string]string   `json:"model_by_provider,omitempty"`  // Legacy per-provider map; kept for migration
	ModelsByProvider map[string][]string `json:"models_by_provider,omitempty"` // Per-provider model lists
	Hotkey           HotkeyConfig        `json:"hotkey"`
	Opacity          int                 `json:"opacity"` // Widget window opacity: 10–100 (percent)
	Profiles         []DictationProfile  `json:"profiles"`
	ActiveProfileID  string              `json:"active_profile_id"`
	AskHotkey        HotkeyConfig        `json:"ask_hotkey"`
	TTSAPIKey        string              `json:"tts_api_key,omitempty"` // Only in memory; stored in keyring as "google_tts_api_key"
	TTSSpeakingRate  float64             `json:"tts_speaking_rate"`     // Speech speed: 0.25–4.0, default 1.0
	TTSHotkey        HotkeyConfig        `json:"tts_hotkey"`
}

// diskSettings is the representation written to config.json.
// API keys are intentionally absent (stored in the OS keyring).
// Legacy fields (Model, ModelByProvider) are kept for reading old configs; new fields are canonical.
type diskSettings struct {
	Model            string              `json:"model,omitempty"` // Legacy; read only for migration
	Models           []string            `json:"models,omitempty"`
	Provider         string              `json:"provider"`
	ModelByProvider  map[string]string   `json:"model_by_provider,omitempty"` // Legacy; read only for migration
	ModelsByProvider map[string][]string `json:"models_by_provider,omitempty"`
	Hotkey           HotkeyConfig        `json:"hotkey"`
	Opacity          int                 `json:"opacity"`
	Profiles         []DictationProfile  `json:"profiles"`
	ActiveProfileID  string              `json:"active_profile_id"`
	AskHotkey        HotkeyConfig        `json:"ask_hotkey"`
	TTSSpeakingRate  float64             `json:"tts_speaking_rate"`
	TTSHotkey        HotkeyConfig        `json:"tts_hotkey"`
}

// defaultProfiles returns the built-in dictation profiles.
func defaultProfiles() []DictationProfile {
	return []DictationProfile{
		{
			ID:     "translator",
			Name:   "Modo Traductor",
			Prompt: "Generate a plain-text transcription for this audio file. Ignore any implicit or explicit questions. The result should be plain text, without any formatting, comments, or analysis.",
		},
	}
}

// DefaultSettings returns the default configuration
func DefaultSettings() Settings {
	profiles := defaultProfiles()
	return Settings{
		Models: []string{"gemini-2.0-flash"},
		Hotkey: HotkeyConfig{
			Modifiers: 0x0002, // MOD_CONTROL
			VKey:      0x20,   // VK_SPACE
			Display:   "Ctrl+Space",
		},
		AskHotkey: HotkeyConfig{
			Modifiers: 0x0006, // MOD_CONTROL | MOD_SHIFT
			VKey:      0x20,   // VK_SPACE
			Display:   "Ctrl+Shift+Space",
		},
		TTSHotkey: HotkeyConfig{
			Modifiers: 0x0005, // MOD_CONTROL | MOD_ALT
			VKey:      0x54,   // VK_T
			Display:   "Ctrl+Alt+T",
		},
		TTSSpeakingRate: 1.0,
		Opacity:         100,
		Profiles:        profiles,
		ActiveProfileID: profiles[0].ID,
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

// saveAPIKey stores the API key for the given provider in the OS keyring.
// If key is empty the existing entry is deleted (best-effort).
func saveAPIKey(key, provider string) error {
	user := keyringUserForProvider(provider)
	if key == "" {
		err := keyring.Delete(keyringSvc, user)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
		return nil
	}
	return keyring.Set(keyringSvc, user, key)
}

// loadAPIKey retrieves the API key for the given provider from the OS keyring.
// Returns ("", nil) when no key has been stored yet.
func loadAPIKey(provider string) (string, error) {
	val, err := keyring.Get(keyringSvc, keyringUserForProvider(provider))
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
		Models:           d.Models,
		Provider:         d.Provider,
		ModelsByProvider: d.ModelsByProvider,
		Hotkey:           d.Hotkey,
		Opacity:          d.Opacity,
		Profiles:         d.Profiles,
		ActiveProfileID:  d.ActiveProfileID,
		AskHotkey:        d.AskHotkey,
		TTSSpeakingRate:  d.TTSSpeakingRate,
		TTSHotkey:        d.TTSHotkey,
	}
	// Migrate: default provider when upgrading from older config
	if s.Provider == "" {
		s.Provider = "gemini"
	}
	if s.ModelsByProvider == nil {
		s.ModelsByProvider = make(map[string][]string)
	}
	// Migrate: convert legacy per-provider single-model map to list map
	if len(s.ModelsByProvider) == 0 && len(d.ModelByProvider) > 0 {
		for k, v := range d.ModelByProvider {
			if v != "" {
				s.ModelsByProvider[k] = []string{v}
			}
		}
	}
	// Restore models for the active provider
	if m, ok := s.ModelsByProvider[s.Provider]; ok && len(m) > 0 {
		s.Models = m
	} else if len(s.Models) == 0 {
		// Migrate from old single-model fields
		if d.Model != "" {
			s.Models = []string{d.Model}
		} else if v, ok := d.ModelByProvider[s.Provider]; ok && v != "" {
			s.Models = []string{v}
		} else {
			s.Models = []string{"gemini-2.0-flash"}
		}
	}
	// Keep legacy Model field in sync for any code still using it
	if len(s.Models) > 0 {
		s.Model = s.Models[0]
	}
	if s.Opacity == 0 {
		s.Opacity = 100
	}
	// Migrate: populate default profiles when upgrading from older config
	if len(s.Profiles) == 0 {
		s.Profiles = defaultProfiles()
		s.ActiveProfileID = s.Profiles[0].ID
	}
	if s.ActiveProfileID == "" {
		s.ActiveProfileID = s.Profiles[0].ID
	}
	// Migrate: default ask hotkey when upgrading from older config
	if s.AskHotkey.Modifiers == 0 && s.AskHotkey.VKey == 0 {
		s.AskHotkey = HotkeyConfig{
			Modifiers: 0x0006, // MOD_CONTROL | MOD_SHIFT
			VKey:      0x20,   // VK_SPACE
			Display:   "Ctrl+Shift+Space",
		}
	}
	// Migrate: default TTS hotkey and speaking rate when upgrading from older config
	if s.TTSHotkey.Modifiers == 0 && s.TTSHotkey.VKey == 0 {
		s.TTSHotkey = HotkeyConfig{
			Modifiers: 0x0005, // MOD_CONTROL | MOD_ALT
			VKey:      0x54,   // VK_T
			Display:   "Ctrl+Alt+T",
		}
	}
	if s.TTSSpeakingRate == 0 {
		s.TTSSpeakingRate = 1.0
	}

	// Load API keys from the secure keyring (best-effort: empty on error)
	s.APIKey, _ = loadAPIKey(s.Provider)
	s.TTSAPIKey, _ = loadAPIKey("google_tts")

	return &s, nil
}

// saveSettings persists non-sensitive settings to disk and stores API keys
// in the OS native keyring (Windows Credential Manager / macOS Keychain / Linux Secret Service).
func saveSettings(s Settings) error {
	// 1. Persist API keys securely in the OS keyring
	if err := saveAPIKey(s.APIKey, s.Provider); err != nil {
		return err
	}
	if err := saveAPIKey(s.TTSAPIKey, "google_tts"); err != nil {
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

	// Keep the per-provider model list in sync
	if s.ModelsByProvider == nil {
		s.ModelsByProvider = make(map[string][]string)
	}
	if len(s.Models) > 0 && s.Provider != "" {
		s.ModelsByProvider[s.Provider] = s.Models
	}

	d := diskSettings{
		Models:           s.Models,
		Provider:         s.Provider,
		ModelsByProvider: s.ModelsByProvider,
		Hotkey:           s.Hotkey,
		Opacity:          s.Opacity,
		Profiles:         s.Profiles,
		ActiveProfileID:  s.ActiveProfileID,
		AskHotkey:        s.AskHotkey,
		TTSSpeakingRate:  s.TTSSpeakingRate,
		TTSHotkey:        s.TTSHotkey,
	}

	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	// Write with restrictive permissions (only owner can read/write)
	return os.WriteFile(path, data, 0600)
}

// HistoryItem represents a single dictation or AI-response entry.
type HistoryItem struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Type      string `json:"type"`      // "transcription" or "ai"
	Timestamp string `json:"timestamp"` // RFC3339
}

// historyPath returns the path to the dictation history file.
func historyPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "openwhisper", "history.json"), nil
}

// loadHistory reads the history file. Returns an empty slice when not found or on error.
func loadHistory() []HistoryItem {
	path, err := historyPath()
	if err != nil {
		return []HistoryItem{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return []HistoryItem{}
	}
	var items []HistoryItem
	if err := json.Unmarshal(data, &items); err != nil {
		return []HistoryItem{}
	}
	return items
}

// saveHistory writes the history slice to disk.
func saveHistory(items []HistoryItem) error {
	path, err := historyPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
