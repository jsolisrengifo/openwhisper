package main

import (
	goruntime "runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

const (
	wmHotkey       = 0x0312
	hotkeyID       = 1
	cancelHotkeyID = 2
	vkEscape       = 0x1B
	pmRemove       = 0x0001
)

// MSG Windows message struct
type MSG struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

// HotkeyManager handles global hotkey registration and listening
type HotkeyManager struct {
	app          *application.App
	widgetWindow *application.WebviewWindow
	quit         chan struct{}
	escQuit      chan struct{} // cancel hotkey goroutine (Escape, only while recording)
	mu           sync.Mutex
	user32       *windows.LazyDLL
}

// NewHotkeyManager creates a new hotkey manager
func NewHotkeyManager(app *application.App, widgetWindow *application.WebviewWindow) *HotkeyManager {
	// escQuit starts pre-closed so StartEscapeListener can detect "not running"
	escQuit := make(chan struct{})
	close(escQuit)
	return &HotkeyManager{
		app:          app,
		widgetWindow: widgetWindow,
		quit:         make(chan struct{}),
		escQuit:      escQuit,
		user32:       windows.NewLazySystemDLL("user32.dll"),
	}
}

// Start registers the given hotkey and listens for it in a polling loop.
// Must be called in a goroutine. Falls back to Ctrl+Space when modifiers/vkey are zero.
func (h *HotkeyManager) Start(modifiers, vkey uint32) {
	// Fallback for zero-value config (e.g. first run after upgrade)
	if modifiers == 0 && vkey == 0 {
		modifiers = 0x0002 // MOD_CONTROL
		vkey = 0x20        // VK_SPACE
	}

	// CRITICAL: Lock to current OS thread so RegisterHotKey messages
	// are delivered to the same thread that polls with PeekMessage.
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	registerHotKey := h.user32.NewProc("RegisterHotKey")
	unregisterHotKey := h.user32.NewProc("UnregisterHotKey")
	peekMessage := h.user32.NewProc("PeekMessageW")

	// MOD_NOREPEAT (0x4000) avoids repeated triggers while held.
	ret, _, _ := registerHotKey.Call(0, hotkeyID, uintptr(modifiers|0x4000), uintptr(vkey))
	if ret == 0 {
		// Fallback: register without MOD_NOREPEAT (Windows 7 compatibility)
		registerHotKey.Call(0, hotkeyID, uintptr(modifiers), uintptr(vkey))
	}
	defer unregisterHotKey.Call(0, hotkeyID)

	h.mu.Lock()
	quit := h.quit
	h.mu.Unlock()

	var msg MSG
	for {
		select {
		case <-quit:
			return
		default:
		}

		// PeekMessage is non-blocking: check if a WM_HOTKEY message is pending
		ret, _, _ := peekMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			wmHotkey,
			wmHotkey,
			pmRemove,
		)

		if ret != 0 && msg.Message == wmHotkey && msg.WParam == hotkeyID {
			h.app.Event.Emit("toggle-recording")
		}

		time.Sleep(30 * time.Millisecond)
	}
}

// Restart re-registers the hotkey with new parameters.
func (h *HotkeyManager) Restart(modifiers, vkey uint32) {
	h.Stop()
	// Give the polling goroutine time to exit (2x poll interval).
	time.Sleep(80 * time.Millisecond)
	h.mu.Lock()
	h.quit = make(chan struct{})
	h.mu.Unlock()
	go h.Start(modifiers, vkey)
}

// Stop signals the hotkey listener to stop
func (h *HotkeyManager) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.quit:
	// already closed
	default:
		close(h.quit)
	}
}

// StartEscapeListener registers Escape as a global hotkey and emits
// "cancel-recording" when pressed. Safe to call multiple times.
func (h *HotkeyManager) StartEscapeListener() {
	h.mu.Lock()
	select {
	case <-h.escQuit:
		// closed = not running, start a new one
		h.escQuit = make(chan struct{})
	default:
		// open = already running
		h.mu.Unlock()
		return
	}
	quit := h.escQuit
	h.mu.Unlock()
	go h.runEscapeListener(quit)
}

// StopEscapeListener unregisters the Escape hotkey.
func (h *HotkeyManager) StopEscapeListener() {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.escQuit:
		// already stopped
	default:
		close(h.escQuit)
	}
}

func (h *HotkeyManager) runEscapeListener(quit <-chan struct{}) {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	registerHotKey := h.user32.NewProc("RegisterHotKey")
	unregisterHotKey := h.user32.NewProc("UnregisterHotKey")
	peekMessage := h.user32.NewProc("PeekMessageW")

	// Register Escape with no modifiers + MOD_NOREPEAT
	registerHotKey.Call(0, cancelHotkeyID, 0x4000, vkEscape)
	defer unregisterHotKey.Call(0, cancelHotkeyID)

	var msg MSG
	for {
		select {
		case <-quit:
			return
		default:
		}

		ret, _, _ := peekMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			wmHotkey,
			wmHotkey,
			pmRemove,
		)

		if ret != 0 && msg.Message == wmHotkey && msg.WParam == cancelHotkeyID {
			h.app.Event.Emit("cancel-recording")
		}

		time.Sleep(30 * time.Millisecond)
	}
}
