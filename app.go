package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App struct
type App struct {
	app            *application.App
	widgetWindow   *application.WebviewWindow
	settingsWindow *application.WebviewWindow
	settings       *Settings
	hotkey         *HotkeyManager
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// shutdown is called when the app is about to quit
func (a *App) shutdown() {
	if a.hotkey != nil {
		a.hotkey.Stop()
	}
}

// TranscribeAudio sends audio base64 to Gemini API and returns transcription
func (a *App) TranscribeAudio(base64Audio string, mimeType string) (string, error) {
	if a.settings == nil || a.settings.APIKey == "" {
		a.app.Event.Emit("open-settings")
		return "", fmt.Errorf("API key no configurada. Por favor configura tu API key de Gemini")
	}
	if base64Audio == "" {
		return "", fmt.Errorf("no se recibió audio")
	}
	return transcribeAudio(base64Audio, mimeType, a.settings.APIKey, a.settings.Model)
}

// PasteText writes text to clipboard and simulates Ctrl+V
func (a *App) PasteText(text string) error {
	a.app.Clipboard.SetText(text)
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

// HideWindow hides the floating widget (used by the \u2212 button)
func (a *App) HideWindow() {
	a.widgetWindow.Hide()
}

// ShowSettingsWindow opens the settings window
func (a *App) ShowSettingsWindow() {
	a.settingsWindow.Show()
	a.settingsWindow.Focus()
	a.app.Event.Emit("settings:show")
}

// HideSettingsWindow closes the settings window
func (a *App) HideSettingsWindow() {
	a.settingsWindow.Hide()
}
