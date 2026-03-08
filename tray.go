package main

import (
	"context"
	_ "embed"
	"runtime"

	"github.com/energye/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// startTray runs the system tray icon. Must be called in a dedicated goroutine.
func startTray(ctx context.Context) {
	runtime.LockOSThread()

	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTooltip("OpenWhisper")

		mShow := systray.AddMenuItem("Mostrar", "Mostrar la ventana")
		mHide := systray.AddMenuItem("Ocultar", "Ocultar la ventana")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Salir", "Cerrar OpenWhisper")

		mShow.Click(func() {
			wailsruntime.WindowShow(ctx)
		})
		mHide.Click(func() {
			wailsruntime.WindowHide(ctx)
		})
		mQuit.Click(func() {
			systray.Quit()
			wailsruntime.Quit(ctx)
		})
	}, func() {
		// onExit
	})
}
