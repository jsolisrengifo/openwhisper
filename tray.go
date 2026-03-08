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

// startTray creates the system tray icon using the Wails v3 native tray.
func startTray(app *application.App, widgetWindow *application.WebviewWindow) {
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("OpenWhisper")

	menu := application.NewMenu()
	menu.Add("Mostrar").OnClick(func(_ *application.Context) {
		widgetWindow.Show()
	})
	menu.Add("Ocultar").OnClick(func(_ *application.Context) {
		widgetWindow.Hide()
	})
	menu.AddSeparator()
	menu.Add("Salir").OnClick(func(_ *application.Context) {
		app.Quit()
	})

	tray.SetMenu(menu)
}
