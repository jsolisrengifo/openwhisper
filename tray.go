package main

import (
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// PNG works reliably with CreateIconFromResourceEx; ICO files cause
// "The operation completed successfully." false-failure on Windows.
//
//go:embed build/appicon.png
var trayIcon []byte

// TrayManager centralises system-tray icon state and interactions.
type TrayManager struct {
	tray  *application.SystemTray
	icons TrayIcons
	app   *application.App
}

// newTrayManager creates the system tray with a right-click menu
// (Configuración, Salir) and a left-click handler that toggles recording.
func newTrayManager(app *application.App) *TrayManager {
	icons := generateTrayIcons()

	tray := app.SystemTray.New()
	tray.SetIcon(icons.Idle)
	tray.SetTooltip("OpenWhisper")

	// Right-click menu
	menu := application.NewMenu()
	menu.Add("Configuración").OnClick(func(_ *application.Context) {
		app.Event.Emit("open-settings")
	})
	menu.AddSeparator()
	menu.Add("Salir").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	// Left-click: toggle recording (same event as the main hotkey)
	tray.OnClick(func() {
		app.Event.Emit("toggle-recording")
	})

	return &TrayManager{tray: tray, icons: icons, app: app}
}

// SetState updates the tray icon and tooltip to reflect the current app state.
func (tm *TrayManager) SetState(state string) {
	var icon []byte
	var tooltip string

	switch state {
	case "recording":
		icon = tm.icons.Recording
		tooltip = "OpenWhisper — Grabando…"
	case "paused":
		icon = tm.icons.Paused
		tooltip = "OpenWhisper — En pausa"
	case "transcribing":
		icon = tm.icons.Transcribing
		tooltip = "OpenWhisper — Transcribiendo…"
	case "done":
		icon = tm.icons.Done
		tooltip = "OpenWhisper — ¡Listo!"
	case "error":
		icon = tm.icons.Error
		tooltip = "OpenWhisper — Error"
	default: // "idle" or unknown
		icon = tm.icons.Idle
		tooltip = "OpenWhisper"
	}

	tm.tray.SetIcon(icon)
	tm.tray.SetTooltip(tooltip)
}
