package main

import (
	"embed"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	closeLog, err := initLogger()
	if err != nil {
		println("failed to init logger:", err.Error())
	}
	defer closeLog()

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
		Width:            190,
		Height:           46,
		MinWidth:         190,
		MinHeight:        46,
		MaxWidth:         190,
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
		Name:                "settings",
		Title:               "OpenWhisper \u2013 Configuraci\u00f3n",
		Width:               700,
		Height:              480,
		MinWidth:            700,
		MinHeight:           480,
		MaxWidth:            700,
		MaxHeight:           480,
		DisableResize:       true,
		MaximiseButtonState: application.ButtonHidden,
		Hidden:              true,
		URL:                 "/settings.html",
		Windows: application.WindowsWindow{
			DisableIcon: true,
		},
	})
	// Intercept the close button (X / Alt+F4): hide instead of destroy,
	// so the window can be re-shown later.
	settingsWindow.RegisterHook(events.Windows.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		settingsWindow.Hide()
	})
	appStruct.settingsWindow = settingsWindow

	settings, err := LoadSettings()
	if err != nil {
		s := DefaultSettings()
		settings = &s
	}
	appStruct.settings = settings

	// Apply the saved opacity once the WebView2 HWND is available.
	// WindowShow fires before WebView2 fully initialises the native handle,
	// so we retry in a goroutine until NativeWindow() returns a non-zero value.
	widgetWindow.RegisterHook(events.Common.WindowShow, func(e *application.WindowEvent) {
		if appStruct.settings == nil {
			return
		}
		opacityPct := appStruct.settings.Opacity
		go func() {
			for i := 0; i < 40; i++ {
				if uintptr(widgetWindow.NativeWindow()) != 0 {
					applyRoundedCorners(uintptr(widgetWindow.NativeWindow()), 190, 46, 10)
					applyWindowOpacity(widgetWindow, opacityPct)
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()
	})

	appStruct.hotkey = NewHotkeyManager(wailsApp, widgetWindow)
	go appStruct.hotkey.Start(settings.Hotkey.Modifiers, settings.Hotkey.VKey)

	startTray(wailsApp, widgetWindow)

	if err := wailsApp.Run(); err != nil {
		println("Error:", err.Error())
	}
}
