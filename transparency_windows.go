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
	procGetWindowRectWin              = modUser32Win.NewProc("GetWindowRect")
	procSetWindowPosWin               = modUser32Win.NewProc("SetWindowPos")
)

// applyRoundedCorners clips the native window to a rounded rectangle so that
// the OS compositor never renders the square corners behind the CSS border-radius.
// Radius should match the CSS border-radius value (in logical px).
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
	// Use GetWindowRect for physical pixel dimensions so that DPI scaling
	// does not leave a transparent strip (logical px != physical px at DPI > 100%).
	type winRECT struct{ Left, Top, Right, Bottom int32 }
	var rc winRECT
	procGetWindowRectWin.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	physW := int(rc.Right - rc.Left)
	physH := int(rc.Bottom - rc.Top)
	if physW <= 0 || physH <= 0 {
		// Fallback to logical dimensions if GetWindowRect fails
		physW, physH = w, h
	}
	// Scale radius proportionally from logical to physical pixels
	physRadius := radius
	if w > 0 {
		physRadius = radius * physW / w
	}

	hRgn, _, err2 := procCreateRoundRectRgnWin.Call(
		0, 0, uintptr(physW), uintptr(physH),
		uintptr(physRadius*2), uintptr(physRadius*2),
	)
	if hRgn == 0 {
		logger.Warn("applyRoundedCorners: CreateRoundRectRgn failed", "err", err2)
		return
	}
	// SetWindowRgn takes ownership; do NOT DeleteObject afterwards.
	procSetWindowRgnWin.Call(hwnd, hRgn, 1 /* bRedraw = TRUE */)
	logger.Debug("applyRoundedCorners: GDI region applied", "hwnd", hwnd, "physW", physW, "physH", physH, "physRadius", physRadius)
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

// enforceTopmost re-applies the TOPMOST flag via SetWindowPos so the widget
// stays above the Windows taskbar even after it steals focus.
func enforceTopmost(w *application.WebviewWindow) {
	if w == nil {
		return
	}
	hwnd := uintptr(w.NativeWindow())
	if hwnd == 0 {
		return
	}
	const (
		hwndTopmost = ^uintptr(0) // HWND_TOPMOST = (HWND)-1
		swpNoMove   = 0x0002
		swpNoSize   = 0x0001
		swpNoActive = 0x0010
	)
	procSetWindowPosWin.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActive)
}
