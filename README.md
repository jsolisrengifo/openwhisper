# OpenWhisper

A lightweight, always-on-top floating dictation app. Press **Ctrl+Space** (or click the mic button) to record your voice, and OpenWhisper will transcribe it using the **Gemini API** and automatically paste the result wherever your cursor is. Press **Ctrl+Shift+Space** to ask a question directly to the AI and get the answer in a floating response window.

Built with [Go](https://go.dev/) + [Wails v3](https://v3.wails.io/)

---

## Features

- **Global hotkey**  `Ctrl+Space` starts/stops recording from any app
- **Ask AI hotkey**  `Ctrl+Alt+A` records a spoken question and shows the AI answer in a floating window
- **Dictation profiles**  create multiple named profiles, each with a custom prompt — switch the active profile at any time
- **Auto-paste**  transcribed text is pasted directly at your cursor position
- **Always on top**  frameless floating window, stays visible over other apps
- **Gemini-powered**  uses Google Gemini API for high-quality transcription and Q&A
- **Minimal UI**  compact widget window, dark theme, no distractions
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

Click the **⚙** button to open settings:

### API / Model (Configuración tab)

| Field | Description |
|-------|-------------|
| **API Key** | Your Google Gemini API key (e.g. `AIzaSy...`) |
| **Model** | Any compatible Gemini model name (e.g. `gemini-2.0-flash`) |

### Keyboard shortcuts (Configuración tab)

| Shortcut | Action | Default |
|----------|--------|---------|
| Activar grabación | Start/stop recording and paste transcription | `Ctrl+Space` |
| Preguntar a la IA | Record a spoken question, show AI answer in floating window | `Ctrl+Shift+Space` |

Both shortcuts are fully customizable — click the key combination to capture a new one.

### Dictation profiles (Perfiles tab)

Each profile defines how the AI processes the audio via a **system prompt**. You can:

- Create as many profiles as you need with **+ Nuevo perfil**
- Edit the name and prompt of each profile inline
- Set one profile as **Activo** — its prompt is used for all transcriptions
- Delete profiles (at least one must remain)

The default profile is **Modo Traductor** with a prompt that produces clean plain-text transcriptions.

Settings are auto-saved 600 ms after the last change. The widget refreshes immediately.

### Storage locations

| Data | Location |
|------|----------|
| Model, hotkeys, profiles | `%APPDATA%\openwhisper\config.json` (Windows) / `~/.config/openwhisper/config.json` (Linux) / `~/Library/Application Support/openwhisper/config.json` (macOS) |
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

If Go functions exposed to the frontend change, regenerate the JS bindings and copy them to `frontend/src/bindings/`:

```bash
wails3 generate bindings openwhisper
# Then copy the generated files to the correct location:
Copy-Item frontend/bindings/openwhisper/* frontend/src/bindings/openwhisper/ -Force
Remove-Item -Recurse -Force frontend/bindings
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
| Transcription / Q&A | Google Gemini API |
| Credential storage | [go-keyring](https://github.com/zalando/go-keyring) (WCM / Keychain / Secret Service) |
| Global hotkeys | Windows `RegisterHotKey` API |
| Auto-paste | Windows `keybd_event` API |
| Task runner | [Task](https://taskfile.dev/) |

---

## License

MIT