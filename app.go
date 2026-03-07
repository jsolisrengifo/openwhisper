package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	settings *Settings
	hotkey   *HotkeyManager
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	settings, err := LoadSettings()
	if err != nil {
		s := DefaultSettings()
		settings = &s
	}
	a.settings = settings

	// Start global hotkey listener Ctrl+Space
	a.hotkey = NewHotkeyManager(ctx)
	go a.hotkey.Start()
}

// shutdown is called when the app is about to quit
func (a *App) shutdown(ctx context.Context) {
	if a.hotkey != nil {
		a.hotkey.Stop()
	}
}

// TranscribeAudio sends audio base64 to Gemini API and returns transcription
func (a *App) TranscribeAudio(base64Audio string, mimeType string) (string, error) {
	if a.settings == nil || a.settings.APIKey == "" {
		runtime.EventsEmit(a.ctx, "open-settings")
		return "", fmt.Errorf("API key no configurada. Por favor configura tu API key de Gemini")
	}
	if base64Audio == "" {
		return "", fmt.Errorf("no se recibió audio")
	}
	return transcribeAudio(base64Audio, mimeType, a.settings.APIKey, a.settings.Model)
}

// PasteText writes text to clipboard and simulates Ctrl+V
func (a *App) PasteText(text string) error {
	runtime.ClipboardSetText(a.ctx, text)
	return pasteViaKeyboard()
}

// GetSettings returns current settings
func (a *App) GetSettings() Settings {
	if a.settings == nil {
		return DefaultSettings()
	}
	return *a.settings
}

// SaveSettings persists settings to disk
func (a *App) SaveSettings(s Settings) error {
	if err := saveSettings(s); err != nil {
		return err
	}
	a.settings = &s
	return nil
}

// SetWindowSize resizes the window (used when toggling settings view)
func (a *App) SetWindowSize(width int, height int) {
	runtime.WindowSetSize(a.ctx, width, height)
}
