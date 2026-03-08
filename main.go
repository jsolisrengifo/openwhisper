package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	appStruct := NewApp()

	wailsApp := application.New(application.Options{
		Name: "OpenWhisper",
		Services: []application.Service{
			application.NewService(appStruct),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		OnShutdown: func() {
			appStruct.shutdown()
		},
	})

	appStruct.app = wailsApp

	widgetWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "widget",
		Width:            240,
		Height:           46,
		MinWidth:         240,
		MinHeight:        46,
		MaxWidth:         240,
		MaxHeight:        46,
		DisableResize:    true,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundColour: application.NewRGBA(18, 18, 18, 255),
		URL:              "/",
		Windows: application.WindowsWindow{
			// WS_EX_TOOLWINDOW (0x80)     → oculta de la barra de tareas y Alt-Tab
			// WS_EX_TOPMOST    (0x08)     → siempre encima
			// WS_EX_CONTROLPARENT (0x10000) → necesario para WebView2
			// Evitamos WS_EX_NOACTIVATE (0x8000000) que usa HiddenOnTaskbar:true
			// en alpha.74 y bloquea el drag al impedir activación de ventana.
			ExStyle:     0x00010088,
			DisableIcon: true,
		},
	})
	appStruct.widgetWindow = widgetWindow

	settingsWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "settings",
		Title:  "OpenWhisper \u2013 Configuraci\u00f3n",
		Width:  400,
		Height: 280,
		Hidden: true,
		URL:    "/settings.html",
		Windows: application.WindowsWindow{
			DisableIcon: true,
		},
	})
	appStruct.settingsWindow = settingsWindow

	settings, err := LoadSettings()
	if err != nil {
		s := DefaultSettings()
		settings = &s
	}
	appStruct.settings = settings

	appStruct.hotkey = NewHotkeyManager(wailsApp)
	go appStruct.hotkey.Start()

	startTray(wailsApp, widgetWindow)

	if err := wailsApp.Run(); err != nil {
		println("Error:", err.Error())
	}
}
