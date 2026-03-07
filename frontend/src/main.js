import './style.css';
import { TranscribeAudio, PasteText, GetSettings, SaveSettings, SetWindowSize } from '../wailsjs/go/main/App';
import { EventsOn, Quit } from '../wailsjs/runtime/runtime';

// ── Estado de la aplicación ────────────────────────────────────────
const MAIN_W = 240, MAIN_H = 46;
const SETTINGS_W = 360, SETTINGS_H = 250;

let mediaRecorder = null;
let audioChunks = [];
let isRecording = false;
let stream = null;
let isConfigured = false;

// ── Referencias DOM ────────────────────────────────────────────────
const micBtn      = document.getElementById('mic-btn');
const statusText  = document.getElementById('status-text');
const mainView    = document.getElementById('main-view');
const settingsView = document.getElementById('settings-view');
const btnSettings = document.getElementById('btn-settings');
const btnClose    = document.getElementById('btn-close');
const btnSave     = document.getElementById('btn-save');
const btnCancel   = document.getElementById('btn-cancel');
const apiKeyInput = document.getElementById('api-key');
const modelInput  = document.getElementById('model-input');
const btnToggleKey = document.getElementById('btn-toggle-key');

// ── Helpers de UI ──────────────────────────────────────────────────
function setState(state, message) {
    const states = ['recording', 'transcribing', 'done', 'error'];
    micBtn.classList.remove(...states);

    if (state) {
        micBtn.classList.add(state);
    }
    statusText.textContent = message || 'Listo';
}

function showMainView() {
    settingsView.classList.add('hidden');
    mainView.classList.remove('hidden');
    SetWindowSize(MAIN_W, MAIN_H);
}

function showSettingsView() {
    mainView.classList.add('hidden');
    settingsView.classList.remove('hidden');
    SetWindowSize(SETTINGS_W, SETTINGS_H);

    // Cargar configuración actual
    GetSettings().then(s => {
        apiKeyInput.value = s.api_key || '';
        modelInput.value  = s.model || '';
    }).catch(() => {});
}

// ── Grabación de audio ─────────────────────────────────────────────
async function startRecording() {
    if (isRecording) return;

    try {
        stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    } catch (err) {
        setState('error', 'Sin micrófono');
        setTimeout(() => setState(null), 3000);
        return;
    }

    audioChunks = [];
    // Intentar webm/opus primero; fallback a audio/webm
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
        ? 'audio/webm;codecs=opus'
        : 'audio/webm';

    mediaRecorder = new MediaRecorder(stream, { mimeType });
    mediaRecorder.ondataavailable = e => {
        if (e.data && e.data.size > 0) audioChunks.push(e.data);
    };
    mediaRecorder.onstop = handleRecordingStop;

    mediaRecorder.start();
    isRecording = true;
    setState('recording', 'Grabando…');
}

function stopRecording() {
    if (!isRecording || !mediaRecorder) return;
    mediaRecorder.stop();
    isRecording = false;
    if (stream) {
        stream.getTracks().forEach(t => t.stop());
        stream = null;
    }
}

async function handleRecordingStop() {
    setState('transcribing', 'Transcribiendo…');

    const blob = new Blob(audioChunks, { type: 'audio/webm' });
    const mimeType = blob.type || 'audio/webm';

    try {
        const base64 = await blobToBase64(blob);
        const text = await TranscribeAudio(base64, mimeType);

        if (!text || text.trim() === '') {
            setState('error', 'Sin resultado');
            setTimeout(() => setState(null), 3000);
            return;
        }

        // Pegar el texto donde esté el cursor
        await PasteText(text.trim());

        setState('done', '¡Pegado!');
        setTimeout(() => setState(null), 2000);
    } catch (err) {
        const msg = (err && err.message) ? err.message : String(err);
        setState('error', msg.length > 30 ? msg.substring(0, 30) + '…' : msg);
        setTimeout(() => setState(null), 5000);
    }
}

function blobToBase64(blob) {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onloadend = () => {
            // reader.result = "data:audio/webm;base64,AAAA..."
            const base64 = reader.result.split(',')[1];
            resolve(base64);
        };
        reader.onerror = reject;
        reader.readAsDataURL(blob);
    });
}

function toggleRecording() {
    if (!settingsView.classList.contains('hidden')) return;
    if (!isConfigured) { showSettingsView(); return; }
    if (isRecording) {
        stopRecording();
    } else {
        startRecording();
    }
}

// ── Event listeners ────────────────────────────────────────────────
micBtn.addEventListener('click', toggleRecording);

btnSettings.addEventListener('click', showSettingsView);

btnClose.addEventListener('click', () => {
    // Detener grabación si está activa antes de salir
    if (isRecording) stopRecording();
    Quit();
});

btnCancel.addEventListener('click', showMainView);

btnToggleKey.addEventListener('click', () => {
    apiKeyInput.type = apiKeyInput.type === 'password' ? 'text' : 'password';
});

btnSave.addEventListener('click', async () => {
    const settings = {
        api_key: apiKeyInput.value.trim(),
        model: modelInput.value.trim(),
    };

    if (!settings.api_key) {
        apiKeyInput.style.borderColor = '#e53935';
        setTimeout(() => { apiKeyInput.style.borderColor = ''; }, 2000);
        return;
    }
    if (!settings.model) {
        modelInput.style.borderColor = '#e53935';
        setTimeout(() => { modelInput.style.borderColor = ''; }, 2000);
        return;
    }

    try {
        await SaveSettings(settings);
        isConfigured = true;
        statusText.classList.remove('warn');
        showMainView();
    } catch (err) {
        alert('Error guardando configuración: ' + err);
    }
});

// Escuchar evento del hotkey global (Ctrl+Space emitido desde Go)
EventsOn('toggle-recording', toggleRecording);

// Verificar configuración al iniciar
GetSettings().then(s => {
    isConfigured = !!(s.api_key && s.model);
    if (!isConfigured) {
        statusText.textContent = '⚙ Config. pendiente';
        statusText.classList.add('warn');
    }
}).catch(() => {
    isConfigured = false;
    statusText.textContent = '⚙ Config. pendiente';
    statusText.classList.add('warn');
});

// Escuchar evento para abrir settings (cuando no hay API key)
EventsOn('open-settings', showSettingsView);

// ── Inicialización ─────────────────────────────────────────────────
GetSettings().then(s => {
    if (!s.api_key) {
        // Primera ejecución: mostrar configuración
        showSettingsView();
    }
}).catch(() => {
    showSettingsView();
});

