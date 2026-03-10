package main

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	vkControl      = 0x11
	vkShift        = 0x10
	vkAlt          = 0x12
	vkLWin         = 0x5B
	vkRWin         = 0x5C
	vkC            = 0x43
	vkV            = 0x56
	keyeventfKeyup = 0x0002
	cfUnicodeText  = 13
)

// pasteViaKeyboard simulates Ctrl+V using the keybd_event Windows API
// to paste from clipboard at the current cursor position.
// A small delay before pressing is needed so the clipboard write settles.
func pasteViaKeyboard() error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	keybdEvent := user32.NewProc("keybd_event")

	// Wait for clipboard to settle
	time.Sleep(80 * time.Millisecond)

	// Ctrl key down
	keybdEvent.Call(vkControl, 0, 0, 0)
	time.Sleep(20 * time.Millisecond)

	// V key down
	keybdEvent.Call(vkV, 0, 0, 0)
	time.Sleep(20 * time.Millisecond)

	// V key up
	keybdEvent.Call(vkV, 0, keyeventfKeyup, 0)
	time.Sleep(20 * time.Millisecond)

	// Ctrl key up
	keybdEvent.Call(vkControl, 0, keyeventfKeyup, 0)

	return nil
}

// copyViaKeyboard simulates Ctrl+C to copy the current selection to clipboard.
// It first releases any modifier keys (Shift, Alt, Win) that the user may still
// be holding from the hotkey combination, to avoid sending Ctrl+Shift+C (DevTools)
// or other unintended shortcuts to the foreground application.
func copyViaKeyboard() error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	keybdEvent := user32.NewProc("keybd_event")

	// Release any modifiers that might be physically held from the hotkey combo
	keybdEvent.Call(vkShift, 0, keyeventfKeyup, 0)
	keybdEvent.Call(vkAlt, 0, keyeventfKeyup, 0)
	keybdEvent.Call(vkLWin, 0, keyeventfKeyup, 0)
	keybdEvent.Call(vkRWin, 0, keyeventfKeyup, 0)
	time.Sleep(30 * time.Millisecond)

	// Ctrl key down
	keybdEvent.Call(vkControl, 0, 0, 0)
	time.Sleep(20 * time.Millisecond)

	// C key down
	keybdEvent.Call(vkC, 0, 0, 0)
	time.Sleep(20 * time.Millisecond)

	// C key up
	keybdEvent.Call(vkC, 0, keyeventfKeyup, 0)
	time.Sleep(20 * time.Millisecond)

	// Ctrl key up
	keybdEvent.Call(vkControl, 0, keyeventfKeyup, 0)

	return nil
}

// readClipboardText reads the current clipboard text content (Unicode).
// Returns ("", nil) when clipboard is empty or contains non-text data.
func readClipboardText() (string, error) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")

	openClipboard := user32.NewProc("OpenClipboard")
	closeClipboard := user32.NewProc("CloseClipboard")
	getClipboardData := user32.NewProc("GetClipboardData")
	globalLock := kernel32.NewProc("GlobalLock")
	globalUnlock := kernel32.NewProc("GlobalUnlock")

	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return "", fmt.Errorf("OpenClipboard failed: %w", err)
	}
	defer closeClipboard.Call()

	h, _, _ := getClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil // no text on clipboard
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		return "", nil
	}
	defer globalUnlock.Call(h)

	// Interpret the pointer as a UTF-16 null-terminated string
	text := windows.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(ptr))[:])
	return text, nil
}
