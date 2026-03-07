package main

import (
	"time"

	"golang.org/x/sys/windows"
)

const (
	vkControl      = 0x11
	vkV            = 0x56
	keyeventfKeyup = 0x0002
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
