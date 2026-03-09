//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	wsExLayered = 0x00080000 // WS_EX_LAYERED
	lwaAlpha    = 0x00000002 // LWA_ALPHA

	// DWMWA_WINDOW_CORNER_POLICY attribute index (Windows 11 22000+)
	dwmwaWindowCornerPolicy = 33
	dwmwcpRound             = 2 // DWMWCP_ROUND
)

var (
	modUser32Win                      = syscall.NewLazyDLL("user32.dll")
	modGdi32Win                       = syscall.NewLazyDLL("gdi32.dll")
	modDwmapiWin                      = syscall.NewLazyDLL("dwmapi.dll")
	procGetWindowLongWin              = modUser32Win.NewProc("GetWindowLongW")
	procSetWindowLongWin              = modUser32Win.NewProc("SetWindowLongW")
	procSetLayeredWindowAttributesWin = modUser32Win.NewProc("SetLayeredWindowAttributes")
	procSetWindowRgnWin               = modUser32Win.NewProc("SetWindowRgn")
	procCreateRoundRectRgnWin         = modGdi32Win.NewProc("CreateRoundRectRgn")
	procDwmSetWindowAttributeWin      = modDwmapiWin.NewProc("DwmSetWindowAttribute")
)

// applyRoundedCorners clips the native window to a rounded rectangle so that
// the OS compositor never renders the square corners behind the CSS border-radius.
// Radius should match the CSS border-radius value (in px).
func applyRoundedCorners(hwnd uintptr, w, h, radius int) {
	// Windows 11+: ask DWM to clip with smooth rounded corners.
	policy := uint32(dwmwcpRound)
	ret, _, err := procDwmSetWindowAttributeWin.Call(
		hwnd,
		uintptr(dwmwaWindowCornerPolicy),
		uintptr(unsafe.Pointer(&policy)),
		4,
	)
	if ret != 0 {
		logger.Warn("applyRoundedCorners: DwmSetWindowAttribute failed", "hresult", ret, "err", err)
	} else {
		logger.Debug("applyRoundedCorners: DWM rounded corners applied", "hwnd", hwnd)
	}

	// Windows 10 fallback (and ensures hit-testing is also clipped):
	// Create a rounded-rectangle GDI region and assign it to the window.
	hRgn, _, err2 := procCreateRoundRectRgnWin.Call(
		0, 0, uintptr(w), uintptr(h),
		uintptr(radius*2), uintptr(radius*2),
	)
	if hRgn == 0 {
		logger.Warn("applyRoundedCorners: CreateRoundRectRgn failed", "err", err2)
		return
	}
	// SetWindowRgn takes ownership; do NOT DeleteObject afterwards.
	procSetWindowRgnWin.Call(hwnd, hRgn, 1 /* bRedraw = TRUE */)
	logger.Debug("applyRoundedCorners: GDI region applied", "hwnd", hwnd, "w", w, "h", h, "radius", radius)
}

// applyWindowOpacity sets the OS-level opacity of the given Wails window.
// opacityPct is clamped to [10, 100]; 100 = fully opaque, 10 = mostly transparent.
func applyWindowOpacity(w *application.WebviewWindow, opacityPct int) {
	if w == nil {
		logger.Warn("applyWindowOpacity: nil window")
		return
	}
	hwnd := uintptr(w.NativeWindow())
	if hwnd == 0 {
		logger.Warn("applyWindowOpacity: HWND is 0, window not ready yet")
		return
	}
	if opacityPct < 10 {
		opacityPct = 10
	}
	if opacityPct > 100 {
		opacityPct = 100
	}

	gwlExStyleVal := -20
	exStyle, _, _ := procGetWindowLongWin.Call(hwnd, uintptr(gwlExStyleVal))

	if exStyle&uintptr(wsExLayered) == 0 {
		procSetWindowLongWin.Call(hwnd, uintptr(gwlExStyleVal), exStyle|uintptr(wsExLayered))
	}

	alpha := uintptr(opacityPct * 255 / 100)
	ret, _, err := procSetLayeredWindowAttributesWin.Call(hwnd, 0, alpha, uintptr(lwaAlpha))
	if ret == 0 {
		logger.Warn("applyWindowOpacity: SetLayeredWindowAttributes failed", "err", err)
	} else {
		logger.Debug("applyWindowOpacity: opacity applied", "hwnd", hwnd, "opacityPct", opacityPct, "alpha", alpha)
	}
}
