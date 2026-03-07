package main

import (
	"context"
	goruntime "runtime"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	modControl = 0x0002
	vkSpace    = 0x20
	wmHotkey   = 0x0312
	hotkeyID   = 1
	pmRemove   = 0x0001
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
	ctx    context.Context
	quit   chan struct{}
	user32 *windows.LazyDLL
}

// NewHotkeyManager creates a new hotkey manager
func NewHotkeyManager(ctx context.Context) *HotkeyManager {
	return &HotkeyManager{
		ctx:    ctx,
		quit:   make(chan struct{}),
		user32: windows.NewLazySystemDLL("user32.dll"),
	}
}

// Start registers Ctrl+Space and listens for it in a polling loop.
// Must be called in a goroutine. Locks to the current OS thread (required
// for Windows message queues to work correctly).
func (h *HotkeyManager) Start() {
	// CRITICAL: Lock to current OS thread so RegisterHotKey messages
	// are delivered to the same thread that polls with PeekMessage.
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	registerHotKey := h.user32.NewProc("RegisterHotKey")
	unregisterHotKey := h.user32.NewProc("UnregisterHotKey")
	peekMessage := h.user32.NewProc("PeekMessageW")

	// Register Ctrl+Space globally.
	// MOD_NOREPEAT (0x4000) avoids repeated triggers while held.
	ret, _, _ := registerHotKey.Call(0, hotkeyID, modControl|0x4000, vkSpace)
	if ret == 0 {
		// Fallback: register without MOD_NOREPEAT (Windows 7 compatibility)
		registerHotKey.Call(0, hotkeyID, modControl, vkSpace)
	}
	defer unregisterHotKey.Call(0, hotkeyID)

	var msg MSG
	for {
		select {
		case <-h.quit:
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
			runtime.EventsEmit(h.ctx, "toggle-recording")
		}

		time.Sleep(30 * time.Millisecond)
	}
}

// Stop signals the hotkey listener to stop
func (h *HotkeyManager) Stop() {
	select {
	case <-h.quit:
		// already closed
	default:
		close(h.quit)
	}
}
