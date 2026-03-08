package main

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
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

	// Start system tray icon
	go startTray(ctx)

	// Remove window from taskbar (show only in system tray)
	go hideFromTaskbar()
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

// HideWindow hides the floating window (used by the − button)
func (a *App) HideWindow() {
	runtime.WindowHide(a.ctx)
}

// WindowPos holds the current window position
type WindowPos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// GetWindowPosition returns the current window top-left coordinates
func (a *App) GetWindowPosition() WindowPos {
	x, y := runtime.WindowGetPosition(a.ctx)
	return WindowPos{X: x, Y: y}
}

// SetWindowPositionAndSize moves and resizes the window atomically
func (a *App) SetWindowPositionAndSize(x, y, w, h int) {
	runtime.WindowSetPosition(a.ctx, x, y)
	runtime.WindowSetSize(a.ctx, w, h)
}

// hideFromTaskbar removes the app button from the Windows taskbar.
// It sets WS_EX_TOOLWINDOW and clears WS_EX_APPWINDOW on the Wails HWND.
func hideFromTaskbar() {
	user32 := windows.NewLazySystemDLL("user32.dll")
	findWindow := user32.NewProc("FindWindowW")
	getWindowLong := user32.NewProc("GetWindowLongPtrW")
	setWindowLong := user32.NewProc("SetWindowLongPtrW")

	className, _ := windows.UTF16PtrFromString("WebviewWindow")
	hwnd, _, _ := findWindow.Call(uintptr(unsafe.Pointer(className)), 0)
	if hwnd == 0 {
		return
	}

	// GWL_EXSTYLE = -20; ^uintptr(19) is -20 in two's complement
	const gwlExStyle = ^uintptr(19)
	const wsExToolWindow uintptr = 0x00000080
	const wsExAppWindow uintptr = 0x00040000

	exStyle, _, _ := getWindowLong.Call(hwnd, gwlExStyle)
	exStyle = (exStyle | wsExToolWindow) &^ wsExAppWindow
	setWindowLong.Call(hwnd, gwlExStyle, exStyle)
}
