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
		Width:            100,
		Height:           22,
		MinWidth:         100,
		MinHeight:        22,
		MaxWidth:         100,
		MaxHeight:        22,
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

	askWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "ask",
		Width:            520,
		Height:           420,
		MinWidth:         520,
		MinHeight:        200,
		MaxWidth:         520,
		DisableResize:    true,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           true,
		BackgroundColour: application.NewRGBA(18, 18, 18, 255),
		URL:              "/ask.html",
		Windows: application.WindowsWindow{
			ExStyle:     0x00010088, // WS_EX_TOOLWINDOW | WS_EX_TOPMOST | WS_EX_CONTROLPARENT
			DisableIcon: true,
		},
	})
	// Intercept close: hide instead of destroy
	askWindow.RegisterHook(events.Windows.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		askWindow.Hide()
	})
	appStruct.askWindow = askWindow

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
					applyRoundedCorners(uintptr(widgetWindow.NativeWindow()), 90, 22, 5)
					applyWindowOpacity(widgetWindow, opacityPct)
					enforceTopmost(widgetWindow)
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()
	})

	// Apply opacity to ask window when it first shows
	askWindow.RegisterHook(events.Common.WindowShow, func(e *application.WindowEvent) {
		if appStruct.settings == nil {
			return
		}
		opacityPct := appStruct.settings.Opacity
		go func() {
			for i := 0; i < 40; i++ {
				if uintptr(askWindow.NativeWindow()) != 0 {
					applyRoundedCorners(uintptr(askWindow.NativeWindow()), 520, 420, 14)
					applyWindowOpacity(askWindow, opacityPct)
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()
	})

	appStruct.hotkey = NewHotkeyManager(wailsApp, widgetWindow)
	appStruct.setupAskContextCapture()
	appStruct.setupTTSContextCapture()
	go appStruct.hotkey.Start(settings.Hotkey.Modifiers, settings.Hotkey.VKey)
	appStruct.hotkey.StartAsk(settings.AskHotkey.Modifiers, settings.AskHotkey.VKey)
	appStruct.hotkey.StartTTS(settings.TTSHotkey.Modifiers, settings.TTSHotkey.VKey)

	startTray(wailsApp, widgetWindow)

	// Periodically re-enforce TOPMOST so the taskbar cannot steal z-order.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			enforceTopmost(widgetWindow)
		}
	}()

	if err := wailsApp.Run(); err != nil {
		println("Error:", err.Error())
	}
}
