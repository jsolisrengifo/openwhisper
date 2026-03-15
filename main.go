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

	// Widget window is kept alive but invisible (1×1 px, hidden).
	// WebView2 needs a browser context for getUserMedia / MediaRecorder.
	widgetWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "widget",
		Width:            1,
		Height:           1,
		MinWidth:         1,
		MinHeight:        1,
		MaxWidth:         1,
		MaxHeight:        1,
		DisableResize:    true,
		Frameless:        true,
		Hidden:           true,
		BackgroundColour: application.NewRGBA(18, 18, 18, 255),
		URL:              "/",
		Windows: application.WindowsWindow{
			// WS_EX_TOOLWINDOW (0x80) → hidden from taskbar and Alt-Tab
			// WS_EX_CONTROLPARENT (0x10000) → needed for WebView2
			ExStyle:     0x00010080,
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

	ttsWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "tts",
		Width:            280,
		Height:           90,
		MinWidth:         280,
		MinHeight:        90,
		MaxWidth:         280,
		MaxHeight:        90,
		DisableResize:    true,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           true,
		BackgroundColour: application.NewRGBA(18, 18, 22, 255),
		URL:              "/tts.html",
		Windows: application.WindowsWindow{
			ExStyle:     0x00010088, // WS_EX_TOOLWINDOW | WS_EX_TOPMOST | WS_EX_CONTROLPARENT
			DisableIcon: true,
		},
	})
	ttsWindow.RegisterHook(events.Windows.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		ttsWindow.Hide()
	})
	appStruct.ttsWindow = ttsWindow

	settings, err := LoadSettings()
	if err != nil {
		s := DefaultSettings()
		settings = &s
	}
	appStruct.settings = settings

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

	// Apply opacity to TTS window when it first shows
	ttsWindow.RegisterHook(events.Common.WindowShow, func(e *application.WindowEvent) {
		if appStruct.settings == nil {
			return
		}
		opacityPct := appStruct.settings.Opacity
		go func() {
			for i := 0; i < 40; i++ {
				if uintptr(ttsWindow.NativeWindow()) != 0 {
					applyRoundedCorners(uintptr(ttsWindow.NativeWindow()), 280, 90, 12)
					applyWindowOpacity(ttsWindow, opacityPct)
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

	trayMgr := newTrayManager(wailsApp)
	appStruct.trayManager = trayMgr

	// Listen for state changes emitted by Widget.svelte and update the tray icon.
	wailsApp.Event.On("widget:state-change", func(e *application.CustomEvent) {
		if state, ok := e.Data.(string); ok {
			trayMgr.SetState(state)
		}
	})

	if err := wailsApp.Run(); err != nil {
		println("Error:", err.Error())
	}
}
