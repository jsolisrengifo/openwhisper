# OpenWhisper

A lightweight, always-on-top floating dictation app. Press **Ctrl+Space** (or click the mic button) to record your voice, and OpenWhisper will transcribe it using your chosen AI provider and automatically paste the result wherever your cursor is. Press **Ctrl+Shift+Space** to ask a question directly to the AI and get the answer in a floating response window.

Built with [Go](https://go.dev/) + [Wails v3](https://v3.wails.io/)

---

## Features

- **Global hotkey**  `Ctrl+Space` starts/stops recording from any app
- **Ask AI hotkey**  `Ctrl+Shift+Space` records a spoken question and shows the AI answer in a floating window
- **In-situ editing**  select text before pressing the Ask hotkey to use it as context — the AI edits or answers in relation to that text
- **Multi-provider**  switch between **Google Gemini** and **OpenRouter** (100+ models); each provider stores its own API key and last-used model independently
- **Dictation profiles**  create multiple named profiles, each with a custom prompt — switch the active profile at any time
- **Auto-paste**  transcribed text is pasted directly at your cursor position
- **Always on top**  frameless floating window, stays visible over other apps
- **Minimal UI**  compact widget window, dark theme, no distractions
- **Secure credential storage**  API keys stored in the OS native keyring, never in plain text
- **Live configuration refresh**  the widget updates instantly when settings are saved

---

## Requirements

- Windows 10/11
- [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (pre-installed on Windows 11)
- An API key for at least one supported provider (see [Configuration](#configuration))

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

OpenWhisper requires an API key and a model to operate. If either is missing when the app starts, the status bar will show **⚙ Config. pendiente** and recording will be blocked until configuration is complete.

Click the **⚙** button to open settings:

### Provider & API Key (Configuración tab)

OpenWhisper supports two AI providers. Use the **Proveedor** dropdown to switch between them.

| Provider | API Key source | Notes |
|----------|---------------|-------|
| **Google Gemini** | [Google AI Studio](https://aistudio.google.com/app/apikey) | Supports audio directly; free tier: 20 req/min |
| **OpenRouter** | [openrouter.ai/keys](https://openrouter.ai/keys) | Proxies 100+ models via OpenAI-compatible API; model must support audio input |

Each provider stores its API key and last-used model independently — switching providers does not overwrite the other provider's settings.

### Model

Enter any model name compatible with the selected provider, e.g.:
- Gemini: `gemini-2.5-flash-lite`, `gemini-2.0-flash`
- OpenRouter: `google/gemini-2.0-flash-001`, `openai/gpt-4o-audio-preview`

> **Note for OpenRouter:** not all models accept raw audio input. Choose a model that explicitly supports audio (`input_audio`) or multimodal content; otherwise the API will return a "No endpoints found that support input audio" error.

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
| Model, provider, hotkeys, profiles | `%APPDATA%\openwhisper\config.json` |
| **API Keys** | **OS native keyring** — Windows Credential Manager |

> **Security note:** API keys are never written to `config.json`. They are stored exclusively in the operating system's secure credential store, isolated from the file system and inaccessible to other user processes without explicit authorization.

---

## Development Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Wails CLI v3](https://v3.wails.io/): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- [Task](https://taskfile.dev/) (task runner): `go install github.com/go-task/task/v3/cmd/task@latest`
- Node.js 18+ (for frontend tooling)

### Install frontend dependencies

```bash
cd frontend
npm install
```

---

## Regenerating Bindings

The JS bindings in `frontend/bindings/` are auto-generated from the Go service layer. They are regenerated automatically as part of the build pipeline (`wails3 task build`).

To regenerate them manually:

```bash
wails3 generate bindings -d frontend/bindings
```

> `frontend/bindings/` is listed in `.gitignore` — do not commit it.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop framework | [Wails v3](https://v3.wails.io/) |
| Backend | Go |
| Frontend | [Svelte 5](https://svelte.dev/) + Vite 5 |
| Runtime bridge | [@wailsio/runtime](https://www.npmjs.com/package/@wailsio/runtime) |
| Renderer | WebView2 (Chromium) |
| Transcription / Q&A | Google Gemini API / OpenRouter API |
| Credential storage | [go-keyring](https://github.com/zalando/go-keyring) (Windows Credential Manager) |
| Global hotkeys | Windows `RegisterHotKey` API |
| Auto-paste | Windows `keybd_event` API |
| Task runner | [Task](https://taskfile.dev/) |

---

## License

MIT