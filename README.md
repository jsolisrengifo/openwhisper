# OpenWhisper

A lightweight, always-on-top floating dictation app for Windows. Press **Ctrl+Space** (or click the mic button) to record your voice, and OpenWhisper will transcribe it using the **Gemini API** and automatically paste the result wherever your cursor is.

Built with [Go](https://go.dev/) + [Wails v2](https://wails.io/)

---

## Features

- **Global hotkey**  `Ctrl+Space` starts/stops recording from any app
- **Auto-paste**  transcribed text is pasted directly at your cursor position
- **Always on top**  frameless floating window, stays visible over other apps
- **Gemini-powered**  uses Google Gemini API for high-quality transcription
- **Minimal UI**  compact window, dark theme, no distractions
- **Persistent settings**  API key and model stored locally in AppData

---

## Requirements

- Windows 10/11
- [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (pre-installed on Windows 11)
- A [Google Gemini API key](https://aistudio.google.com/app/apikey)

---

## Getting Started

### Run in development mode

```bash
wails dev
```

### Build a production binary

```bash
wails build
```

The compiled `.exe` will be placed in `build/bin/`.

---

## Configuration

OpenWhisper requires two fields to operate. If either is missing when the app starts, the status bar will show **⚙ Config. pendiente** and recording will be blocked until configuration is complete.

Click the **⚙** button to open settings and fill in:

| Field | Description |
|-------|-------------|
| **API Key** | Your Google Gemini API key (e.g. `AIzaSy...`) |
| **Model** | Any compatible Gemini model name (e.g. `gemini-2.0-flash`) |

Both fields are required. The app has no hardcoded defaults — you choose the model.

Settings are saved to:
```
%APPDATA%\openwhisper\config.json
```

---

## Development Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Node.js (for frontend tooling, managed by Wails)

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop framework | [Wails v2](https://wails.io/) |
| Backend | Go |
| Frontend | Vanilla JS + Vite |
| Renderer | WebView2 (Chromium) |
| Transcription | Google Gemini API |
| Global hotkey | Windows `RegisterHotKey` API |
| Auto-paste | Windows `keybd_event` API |

---

## License

MIT