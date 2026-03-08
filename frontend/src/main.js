import './style.css';
import { TranscribeAudio, PasteText, ShowSettingsWindow, HideWindow, GetSettings } from './bindings/openwhisper/app.js';
import { Events } from '@wailsio/runtime';

//  Estado
let mediaRecorder = null;
let audioChunks = [];
let isRecording = false;
let stream = null;
let isConfigured = false;

//  Referencias DOM
const micBtn      = document.getElementById('mic-btn');
const statusText  = document.getElementById('status-text');
const btnSettings = document.getElementById('btn-settings');
const btnClose    = document.getElementById('btn-close');

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

    if (!isConfigured) { ShowSettingsWindow(); return; }

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
        setState('done', '\u00a1Pegado!');
        setTimeout(() => setState(null), 2000);
    } catch (err) {
        const msg = (err && err.message) ? err.message : String(err);
        setState('error', msg.length > 30 ? msg.substring(0, 30) + '\u2026' : msg);
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
btnSettings.addEventListener('click', () => ShowSettingsWindow());
btnClose.addEventListener('click', () => {
    if (isRecording) stopRecording();
    HideWindow();
});

//  Eventos desde el backend Go
Events.On('toggle-recording', toggleRecording);
Events.On('open-settings', () => ShowSettingsWindow());

//  Init: comprobar si la API key está configurada

GetSettings().then(s => {
    isConfigured = !!(s.api_key && s.model);
    if (!isConfigured) setUnconfigured();
}).catch(() => {
    isConfigured = false;
    setUnconfigured();
});
