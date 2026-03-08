# OpenWhisper

A lightweight, always-on-top floating dictation app. Press **Ctrl+Space** (or click the mic button) to record your voice, and OpenWhisper will transcribe it using the **Gemini API** and automatically paste the result wherever your cursor is.

Built with [Go](https://go.dev/) + [Wails v3](https://v3.wails.io/)

---

## Features

- **Global hotkey**  `Ctrl+Space` starts/stops recording from any app
- **Auto-paste**  transcribed text is pasted directly at your cursor position
- **Always on top**  frameless floating window, stays visible over other apps
- **Gemini-powered**  uses Google Gemini API for high-quality transcription
- **Minimal UI**  compact window, dark theme, no distractions
- **Secure credential storage**  API key stored in the OS native keyring, never in plain text
- **Live configuration refresh**  the widget updates instantly when settings are saved

---

## Requirements

- Windows 10/11 (macOS and Linux also supported)
- [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (pre-installed on Windows 11; Windows only)
- A [Google Gemini API key](https://aistudio.google.com/app/apikey)

---

## Getting Started

### Run in development mode

```bash
wails3 task dev
```

Or directly:

```bash
wails3 dev -config ./build/config.yml
```

### Build a production binary

```bash
wails3 task build
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

Settings are auto-saved 600 ms after the last keystroke. The floating widget refreshes immediately — the "⚙ Config. pendiente" indicator disappears as soon as a valid API key and model are saved, with no need to restart the app.

### Storage locations

| Data | Location |
|------|----------|
| Model, hotkey | `%APPDATA%\openwhisper\config.json` (Windows) / `~/.config/openwhisper/config.json` (Linux) / `~/Library/Application Support/openwhisper/config.json` (macOS) |
| **API Key** | **OS native keyring** — Windows Credential Manager / macOS Keychain / Linux Secret Service |

> **Security note:** The API key is never written to `config.json`. It is stored exclusively in the operating system's secure credential store, isolated from the file system and inaccessible to other user processes without explicit authorization.

---

## Development Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Wails CLI v3](https://v3.wails.io/): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- [Task](https://taskfile.dev/) (task runner): `go install github.com/go-task/task/v3/cmd/task@latest`
- Node.js 18+ (for frontend tooling)
- **Linux only:** a running `gnome-keyring` or compatible Secret Service daemon

### Install frontend dependencies

```bash
cd frontend
npm install
```

---

## Regenerating Bindings

If Go functions exposed to the frontend change, regenerate the JS bindings:

```bash
wails3 generate bindings -d frontend/src/bindings -clean=true
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop framework | [Wails v3](https://v3.wails.io/) |
| Backend | Go |
| Frontend | [Svelte 5](https://svelte.dev/) + Vite 5 |
| Runtime bridge | [@wailsio/runtime](https://www.npmjs.com/package/@wailsio/runtime) |
| Renderer | WebView2 (Chromium) |
| Transcription | Google Gemini API |
| Credential storage | [go-keyring](https://github.com/zalando/go-keyring) (WCM / Keychain / Secret Service) |
| Global hotkey | Windows `RegisterHotKey` API |
| Auto-paste | Windows `keybd_event` API |
| Task runner | [Task](https://taskfile.dev/) |

---

## License

MIT