package main

import (
	"fmt"
	"log/slog"
	"time"

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
		logger.Warn("TranscribeAudio: no API key configured")
		a.app.Event.Emit("open-settings")
		return "", fmt.Errorf("API key no configurada. Por favor configura tu API key de Gemini")
	}
	if base64Audio == "" {
		logger.Warn("TranscribeAudio: empty audio received")
		return "", fmt.Errorf("no se recibió audio")
	}
	logger.Debug("TranscribeAudio: starting", "mimeType", mimeType)
	start := time.Now()
	result, err := transcribeAudio(base64Audio, mimeType, a.settings.APIKey, a.settings.Model)
	if err != nil {
		logger.Error("TranscribeAudio: failed", "err", err, "elapsed", time.Since(start).String())
		return "", err
	}
	return result, nil
}

// EnableCancelHotkey registers Escape as a global hotkey to cancel recording.
func (a *App) EnableCancelHotkey() {
	if a.hotkey != nil {
		a.hotkey.StartEscapeListener()
	}
}

// DisableCancelHotkey unregisters the Escape hotkey.
func (a *App) DisableCancelHotkey() {
	if a.hotkey != nil {
		a.hotkey.StopEscapeListener()
	}
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
	logger.Debug("SaveSettings: saving", "model", s.Model, "opacity", s.Opacity, "hotkey", s.Hotkey.Display)
	if err := saveSettings(s); err != nil {
		logger.Error("SaveSettings: write failed", "err", err)
		return err
	}
	// Restart hotkey listener if the shortcut changed
	if a.hotkey != nil && a.settings != nil {
		if s.Hotkey.Modifiers != a.settings.Hotkey.Modifiers || s.Hotkey.VKey != a.settings.Hotkey.VKey {
			a.hotkey.Restart(s.Hotkey.Modifiers, s.Hotkey.VKey)
		}
	}
	a.settings = &s
	// Apply opacity change to the widget window
	applyWindowOpacity(a.widgetWindow, s.Opacity)
	// Notify all windows that settings have been updated
	a.app.Event.Emit("settings:saved")
	logger.Info("SaveSettings: saved ok", slog.Int("opacity", s.Opacity), slog.String("hotkey", s.Hotkey.Display))
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
