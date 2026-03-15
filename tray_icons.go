package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// TrayIcons holds pre-rendered PNG icon variants for each app state.
type TrayIcons struct {
	Idle         []byte // original icon, no overlay
	Recording    []byte // red dot
	Paused       []byte // amber dot
	Transcribing []byte // blue dot
	Done         []byte // green dot
	Error        []byte // orange dot
}

// generateTrayIcons decodes the base PNG (appicon.png embedded via trayIcon)
// and produces colour-dot variants for every visual state.
func generateTrayIcons() TrayIcons {
	base, err := png.Decode(bytes.NewReader(trayIcon))
	if err != nil {
		// If the icon can't be decoded, fall back to the raw bytes for every state.
		return TrayIcons{
			Idle: trayIcon, Recording: trayIcon, Paused: trayIcon,
			Transcribing: trayIcon, Done: trayIcon, Error: trayIcon,
		}
	}

	return TrayIcons{
		Idle:         trayIcon,                                                         // no overlay
		Recording:    overlayDot(base, color.RGBA{R: 0xe5, G: 0x39, B: 0x35, A: 0xff}), // #e53935
		Paused:       overlayDot(base, color.RGBA{R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff}), // #f59e0b
		Transcribing: overlayDot(base, color.RGBA{R: 0x19, G: 0x76, B: 0xd2, A: 0xff}), // #1976d2
		Done:         overlayDot(base, color.RGBA{R: 0x4c, G: 0xaf, B: 0x50, A: 0xff}), // #4caf50
		Error:        overlayDot(base, color.RGBA{R: 0xf5, G: 0x7f, B: 0x17, A: 0xff}), // #f57f17
	}
}

// overlayDot draws a filled circle (≈25 % of image width) at the bottom-right
// corner of the base image and returns the composited PNG bytes.
func overlayDot(base image.Image, c color.Color) []byte {
	b := base.Bounds()
	w, h := b.Dx(), b.Dy()

	// Copy base into a mutable RGBA image.
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, base, b.Min, draw.Src)

	// Dot parameters: radius ≈ 25 % of width, centred at bottom-right.
	radius := float64(w) * 0.25
	if radius < 2 {
		radius = 2
	}
	cx := float64(b.Min.X) + float64(w) - radius - 1
	cy := float64(b.Min.Y) + float64(h) - radius - 1

	// Thin dark border around the dot for contrast.
	borderR := radius + 1
	fillCircle(dst, cx, cy, borderR, color.RGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xff})
	fillCircle(dst, cx, cy, radius, c)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return trayIcon // fallback
	}
	return buf.Bytes()
}

// fillCircle fills a circle on dst using brute-force pixel iteration.
func fillCircle(dst *image.RGBA, cx, cy, r float64, c color.Color) {
	b := dst.Bounds()
	r2 := r * r
	minX := int(math.Floor(cx - r))
	maxX := int(math.Ceil(cx + r))
	minY := int(math.Floor(cy - r))
	maxY := int(math.Ceil(cy + r))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
				continue
			}
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r2 {
				dst.Set(x, y, c)
			}
		}
	}
}
