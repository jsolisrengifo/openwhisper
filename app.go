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
	askWindow      *application.WebviewWindow
	settings       *Settings
	hotkey         *HotkeyManager

	// Ask AI context capture — set by setupAskContextCapture, consumed by AskAI
	pendingContextText string

	// isAskActive is true between the first hotkey press (start recording) and AskAI completion.
	// Used to distinguish the start press (capture context) from the stop press (skip capture).
	isAskActive bool

	// Last Ask AI request — used by RegenerateAsk
	lastAskAudio   string
	lastAskMime    string
	lastAskContext string // non-empty when last ask was an in-situ edit
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

	// Look up the active profile's prompt; fall back to the first profile or a hardcoded default.
	prompt := "Generate a plain-text transcription for this audio file. The result should be plain text, without any formatting."
	for _, p := range a.settings.Profiles {
		if p.ID == a.settings.ActiveProfileID {
			prompt = p.Prompt
			break
		}
	}
	if len(a.settings.Profiles) > 0 && prompt == "Generate a plain-text transcription for this audio file. The result should be plain text, without any formatting." {
		// ActiveProfileID may not match; fall back to first profile
		prompt = a.settings.Profiles[0].Prompt
	}

	logger.Debug("TranscribeAudio: starting", "mimeType", mimeType, "profile", a.settings.ActiveProfileID)
	start := time.Now()
	result, err := transcribeAudio(base64Audio, mimeType, a.settings.APIKey, a.settings.Models, prompt, a.settings.Provider)
	if err != nil {
		logger.Error("TranscribeAudio: failed", "err", err, "elapsed", time.Since(start).String())
		return "", err
	}
	return result, nil
}

// AskAI sends audio to Gemini and returns an AI-generated answer (not a transcription).
// If text was selected before the hotkey was pressed, it is used as in-situ editing context.
func (a *App) AskAI(base64Audio string, mimeType string) (string, error) {
	if a.settings == nil || a.settings.APIKey == "" {
		logger.Warn("AskAI: no API key configured")
		a.app.Event.Emit("open-settings")
		return "", fmt.Errorf("API key no configurada. Por favor configura tu API key de Gemini")
	}
	if base64Audio == "" {
		logger.Warn("AskAI: empty audio received")
		return "", fmt.Errorf("no se recibió audio")
	}

	// Consume any pending clipboard context captured by setupAskContextCapture
	ctxText := a.pendingContextText
	a.pendingContextText = ""
	a.isAskActive = false // recording is done, ready for next ask

	// Persist for RegenerateAsk
	a.lastAskAudio = base64Audio
	a.lastAskMime = mimeType
	a.lastAskContext = ctxText

	logger.Debug("AskAI: starting", "mimeType", mimeType, "hasContext", ctxText != "")
	start := time.Now()

	var result string
	var err error
	if ctxText != "" {
		result, err = editTextWithAudio(base64Audio, mimeType, a.settings.APIKey, a.settings.Models, ctxText, a.settings.Provider)
	} else {
		result, err = askQuestion(base64Audio, mimeType, a.settings.APIKey, a.settings.Models, a.settings.Provider)
	}
	if err != nil {
		logger.Error("AskAI: failed", "err", err, "elapsed", time.Since(start).String())
		return "", err
	}
	return result, nil
}

// RegenerateAsk re-sends the last AskAI request without re-recording.
func (a *App) RegenerateAsk() (string, error) {
	if a.settings == nil || a.settings.APIKey == "" {
		return "", fmt.Errorf("API key no configurada")
	}
	if a.lastAskAudio == "" {
		return "", fmt.Errorf("no hay una consulta previa para regenerar")
	}
	logger.Debug("RegenerateAsk: re-sending last ask", "hasContext", a.lastAskContext != "")
	start := time.Now()
	var result string
	var err error
	if a.lastAskContext != "" {
		result, err = editTextWithAudio(a.lastAskAudio, a.lastAskMime, a.settings.APIKey, a.settings.Models, a.lastAskContext, a.settings.Provider)
	} else {
		result, err = askQuestion(a.lastAskAudio, a.lastAskMime, a.settings.APIKey, a.settings.Models, a.settings.Provider)
	}
	if err != nil {
		logger.Error("RegenerateAsk: failed", "err", err, "elapsed", time.Since(start).String())
		return "", err
	}
	return result, nil
}

// GetAPIKeyForProvider loads the stored API key for the given provider from the OS keyring.
// Called by the frontend when the user switches providers in settings.
func (a *App) GetAPIKeyForProvider(provider string) (string, error) {
	return loadAPIKey(provider)
}

// CopyText places text on the clipboard without simulating Ctrl+V.
func (a *App) CopyText(text string) {
	a.app.Clipboard.SetText(text)
}

// AskFollowUp sends a follow-up audio question with the full conversation history.
func (a *App) AskFollowUp(base64Audio string, mimeType string, history []ChatTurn) (string, error) {
	if a.settings == nil || a.settings.APIKey == "" {
		return "", fmt.Errorf("API key no configurada")
	}
	if base64Audio == "" {
		return "", fmt.Errorf("no se recibió audio")
	}
	logger.Debug("AskFollowUp: starting", "mimeType", mimeType, "historyLen", len(history))
	start := time.Now()
	result, err := continueChat(base64Audio, mimeType, a.settings.APIKey, a.settings.Models, history, a.settings.Provider)
	if err != nil {
		logger.Error("AskFollowUp: failed", "err", err, "elapsed", time.Since(start).String())
		return "", err
	}
	return result, nil
}

// setupAskContextCapture assigns an OnBeforeAsk hook to the hotkey manager.
// When the ask hotkey fires to START recording, it simulates Ctrl+C and captures
// any newly-copied text as context for the AI. On the STOP press it skips capture
// to avoid overwriting the context that was already saved.
func (a *App) setupAskContextCapture() {
	if a.hotkey == nil {
		return
	}
	a.hotkey.OnBeforeAsk = func() {
		if a.isAskActive {
			// Second press = stop recording — don't touch pendingContextText.
			// isAskActive is reset by AskAI when it consumes the result.
			return
		}
		// First press = start recording — capture any selected text via Ctrl+C.
		a.isAskActive = true
		prev, _ := readClipboardText()
		_ = copyViaKeyboard()
		time.Sleep(200 * time.Millisecond)
		after, _ := readClipboardText()
		if after != "" && after != prev {
			a.pendingContextText = after
			logger.Debug("setupAskContextCapture: captured selection", "chars", len(after))
		} else {
			a.pendingContextText = ""
		}
	}
}

// ShowAnswer displays the AI answer in the floating ask window.
func (a *App) ShowAnswer(text string) {
	if a.askWindow == nil {
		return
	}
	a.askWindow.Show()
	a.askWindow.Center()
	a.askWindow.Focus()
	a.app.Event.Emit("ask:response", text)
}

// HideAskWindow hides the ask/answer window.
func (a *App) HideAskWindow() {
	if a.askWindow != nil {
		a.askWindow.Hide()
	}
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
	// Restart hotkeys if shortcuts changed
	if a.hotkey != nil && a.settings != nil {
		if s.Hotkey.Modifiers != a.settings.Hotkey.Modifiers || s.Hotkey.VKey != a.settings.Hotkey.VKey {
			a.hotkey.Restart(s.Hotkey.Modifiers, s.Hotkey.VKey)
		}
		if s.AskHotkey.Modifiers != a.settings.AskHotkey.Modifiers || s.AskHotkey.VKey != a.settings.AskHotkey.VKey {
			a.hotkey.RestartAsk(s.AskHotkey.Modifiers, s.AskHotkey.VKey)
		}
		if s.TTSHotkey.Modifiers != a.settings.TTSHotkey.Modifiers || s.TTSHotkey.VKey != a.settings.TTSHotkey.VKey {
			a.hotkey.RestartTTS(s.TTSHotkey.Modifiers, s.TTSHotkey.VKey)
		}
	}
	a.settings = &s
	// Apply opacity change to the widget window
	applyWindowOpacity(a.widgetWindow, s.Opacity)
	applyWindowOpacity(a.askWindow, s.Opacity)
	// Notify all windows that settings have been updated
	a.app.Event.Emit("settings:saved")
	logger.Info("SaveSettings: saved ok", slog.Int("opacity", s.Opacity), slog.String("hotkey", s.Hotkey.Display))
	return nil
}

// HideWindow hides the floating widget (used by the − button)
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

// AddHistoryItem prepends a new entry to the dictation history (max 10 items).
func (a *App) AddHistoryItem(text string, itemType string) {
	items := loadHistory()
	newItem := HistoryItem{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Text:      text,
		Type:      itemType,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	items = append([]HistoryItem{newItem}, items...)
	if len(items) > 10 {
		items = items[:10]
	}
	_ = saveHistory(items)
}

// GetHistory returns the dictation history list.
func (a *App) GetHistory() []HistoryItem {
	return loadHistory()
}

// PasteFromHistory hides the settings window, then pastes the given text.
// The brief delay lets the previously focused window regain focus before Ctrl+V fires.
func (a *App) PasteFromHistory(text string) error {
	if a.settingsWindow != nil {
		a.settingsWindow.Hide()
	}
	time.Sleep(300 * time.Millisecond)
	a.app.Clipboard.SetText(text)
	return pasteViaKeyboard()
}

// ClearHistory removes all entries from the dictation history.
func (a *App) ClearHistory() error {
	return saveHistory([]HistoryItem{})
}

// GetTTSAPIKey loads the Google Cloud TTS API key from the OS keyring.
func (a *App) GetTTSAPIKey() (string, error) {
	return loadAPIKey("google_tts")
}

// SpeakText calls the Google Cloud TTS API with the given text and returns the base64 MP3 audio.
// Called from the frontend when a user wants to play a TTS response from the Ask window.
func (a *App) SpeakText(text string) (string, error) {
	if a.settings == nil || a.settings.TTSAPIKey == "" {
		logger.Warn("SpeakText: no TTS API key configured")
		a.app.Event.Emit("open-settings")
		return "", fmt.Errorf("API key de TTS no configurada. Por favor configura tu API key de Google Cloud TTS")
	}
	if text == "" {
		return "", fmt.Errorf("no hay texto para reproducir")
	}
	rate := a.settings.TTSSpeakingRate
	if rate == 0 {
		rate = 1.0
	}
	logger.Debug("SpeakText: starting", "chars", len(text), "speakingRate", rate)
	audio, err := synthesizeSpeech(text, a.settings.TTSAPIKey, rate)
	if err != nil {
		logger.Error("SpeakText: synthesis failed", "err", err)
		return "", err
	}
	return audio, nil
}

// setupTTSContextCapture assigns an OnBeforeTTS hook to the hotkey manager.
// When the TTS hotkey fires, it captures selected text via Ctrl+C, then calls
// Google Cloud TTS and emits the result to the Ask window for playback.
func (a *App) setupTTSContextCapture() {
	if a.hotkey == nil {
		return
	}
	a.hotkey.OnBeforeTTS = func() {
		if a.settings == nil || a.settings.TTSAPIKey == "" {
			logger.Warn("TTS hotkey: no API key configured")
			a.app.Event.Emit("open-settings")
			return
		}
		// Capture currently selected text via Ctrl+C
		prev, _ := readClipboardText()
		_ = copyViaKeyboard()
		time.Sleep(200 * time.Millisecond)
		after, _ := readClipboardText()

		textToSpeak := ""
		if after != "" && after != prev {
			textToSpeak = after
		}
		if textToSpeak == "" {
			logger.Warn("TTS hotkey: no text selected")
			a.app.Event.Emit("tts:error", "No hay texto seleccionado para reproducir")
			return
		}

		logger.Debug("TTS hotkey: captured text", "chars", len(textToSpeak))
		// Show Ask window and signal processing state
		a.askWindow.Show()
		a.askWindow.Center()
		a.askWindow.Focus()
		a.app.Event.Emit("ask:new-chat")
		a.app.Event.Emit("tts:processing")

		rate := a.settings.TTSSpeakingRate
		if rate == 0 {
			rate = 1.0
		}
		ttsKey := a.settings.TTSAPIKey

		// Run TTS in a background goroutine so the hotkey thread is not blocked.
		go func() {
			audio, err := synthesizeSpeech(textToSpeak, ttsKey, rate)
			if err != nil {
				logger.Error("TTS hotkey: synthesis failed", "err", err)
				a.app.Event.Emit("tts:error", err.Error())
				return
			}
			a.app.Event.Emit("tts:audio", audio)
		}()
	}
}
