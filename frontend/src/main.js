import './style.css';
import { TranscribeAudio, PasteText, GetSettings, SaveSettings, SetWindowSize, HideWindow, GetWindowPosition, SetWindowPositionAndSize } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

//  Dimensiones 
const BAR_W = 240, BAR_H = 46;
const PANEL_H = 270; // barra + panel abierto

//  Estado 
let mediaRecorder = null;
let audioChunks = [];
let isRecording = false;
let stream = null;
let panelOpen = false;
let panelOpenedUpward = false;
let isConfigured = false;

//  Referencias DOM 
const micBtn        = document.getElementById('mic-btn');
const statusText    = document.getElementById('status-text');
const btnSettings   = document.getElementById('btn-settings');
const btnClose      = document.getElementById('btn-close');
const btnSave       = document.getElementById('btn-save');
const btnCancel     = document.getElementById('btn-cancel');
const apiKeyInput   = document.getElementById('api-key');
const modelInput    = document.getElementById('model-input');
const btnToggleKey  = document.getElementById('btn-toggle-key');
const settingsPanel = document.getElementById('settings-panel');

//  Panel toggle 
async function openPanel() {
    panelOpen = true;
    settingsPanel.classList.add('open');
    btnSettings.classList.add('active');

    const TASKBAR_H = 48; // espacio reservado para la taskbar
    const pos = await GetWindowPosition();
    const screenH = window.screen.height;
    const spaceBelow = screenH - pos.y - BAR_H - TASKBAR_H;

    if (spaceBelow < (PANEL_H - BAR_H)) {
        // No hay espacio abajo: mover ventana hacia arriba
        panelOpenedUpward = true;
        SetWindowPositionAndSize(pos.x, pos.y - (PANEL_H - BAR_H), BAR_W, PANEL_H);
    } else {
        panelOpenedUpward = false;
        SetWindowSize(BAR_W, PANEL_H);
    }

    GetSettings().then(s => {
        apiKeyInput.value = s.api_key || '';
        modelInput.value  = s.model  || '';
    }).catch(() => {});
}

async function closePanel() {
    panelOpen = false;
    settingsPanel.classList.remove('open');
    btnSettings.classList.remove('active');

    if (panelOpenedUpward) {
        // Restaurar posición original (bajar la ventana de vuelta)
        const pos = await GetWindowPosition();
        SetWindowPositionAndSize(pos.x, pos.y + (PANEL_H - BAR_H), BAR_W, BAR_H);
        panelOpenedUpward = false;
    } else {
        SetWindowSize(BAR_W, BAR_H);
    }
}

function togglePanel() {
    if (panelOpen) { closePanel(); } else { openPanel(); }
}

//  Estado visual 
function setState(state, message) {
    const states = ['recording', 'transcribing', 'done', 'error'];
    micBtn.classList.remove(...states);
    if (state) micBtn.classList.add(state);
    statusText.textContent = message || 'Listo';
    statusText.classList.toggle('warn', false);
}

function setUnconfigured() {
    statusText.textContent = '\u2699 Config. pendiente';
    statusText.classList.add('warn');
}

//  Grabación de audio 
async function startRecording() {
    if (isRecording) return;

    if (!isConfigured) { openPanel(); return; }

    try {
        stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    } catch (err) {
        setState('error', 'Sin micrófono');
        setTimeout(() => setState(null), 3000);
        return;
    }

    audioChunks = [];
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
    setState('recording', 'Grabando');
}

function stopRecording() {
    if (!isRecording || !mediaRecorder) return;
    mediaRecorder.stop();
    isRecording = false;
    if (stream) { stream.getTracks().forEach(t => t.stop()); stream = null; }
}

async function handleRecordingStop() {
    setState('transcribing', 'Transcribiendo');
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

        await PasteText(text.trim());
        setState('done', '¡Pegado!');
        setTimeout(() => setState(null), 2000);
    } catch (err) {
        const msg = (err && err.message) ? err.message : String(err);
        setState('error', msg.length > 30 ? msg.substring(0, 30) + '' : msg);
        setTimeout(() => setState(null), 5000);
    }
}

function blobToBase64(blob) {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onloadend = () => resolve(reader.result.split(',')[1]);
        reader.onerror = reject;
        reader.readAsDataURL(blob);
    });
}

function toggleRecording() {
    if (isRecording) { stopRecording(); } else { startRecording(); }
}

//  Event listeners 
micBtn.addEventListener('click', toggleRecording);
btnSettings.addEventListener('click', togglePanel);

btnClose.addEventListener('click', () => {
    if (isRecording) stopRecording();
    HideWindow();
});

btnCancel.addEventListener('click', closePanel);

btnToggleKey.addEventListener('click', () => {
    apiKeyInput.type = apiKeyInput.type === 'password' ? 'text' : 'password';
});

btnSave.addEventListener('click', async () => {
    const settings = {
        api_key: apiKeyInput.value.trim(),
        model:   modelInput.value.trim(),
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
        closePanel();
        setState(null);
    } catch (err) {
        alert('Error guardando configuración: ' + err);
    }
});

//  Hotkey global 
EventsOn('toggle-recording', toggleRecording);
EventsOn('open-settings', openPanel);

//  Init: verificar configuración 
GetSettings().then(s => {
    isConfigured = !!(s.api_key && s.model);
    if (!isConfigured) setUnconfigured();
}).catch(() => {
    isConfigured = false;
    setUnconfigured();
});