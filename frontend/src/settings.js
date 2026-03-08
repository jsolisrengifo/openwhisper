import { GetSettings, SaveSettings, HideSettingsWindow } from './bindings/openwhisper/app.js';

const apiKeyInput  = document.getElementById('api-key');
const modelInput   = document.getElementById('model-input');
const btnSave      = document.getElementById('btn-save');
const btnCancel    = document.getElementById('btn-cancel');
const btnToggleKey = document.getElementById('btn-toggle-key');
const statusMsg    = document.getElementById('status-msg');

// Cargar los valores actuales al abrir la ventana
GetSettings().then(s => {
    apiKeyInput.value = s.api_key || '';
    modelInput.value  = s.model  || 'gemini-2.0-flash';
}).catch(() => {});

btnToggleKey.addEventListener('click', () => {
    apiKeyInput.type = apiKeyInput.type === 'password' ? 'text' : 'password';
});

btnSave.addEventListener('click', async () => {
    statusMsg.textContent = '';
    const apiKey = apiKeyInput.value.trim();
    const model  = modelInput.value.trim();

    if (!apiKey) {
        apiKeyInput.style.borderColor = '#e53935';
        setTimeout(() => { apiKeyInput.style.borderColor = ''; }, 2000);
        return;
    }
    if (!model) {
        modelInput.style.borderColor = '#e53935';
        setTimeout(() => { modelInput.style.borderColor = ''; }, 2000);
        return;
    }

    try {
        await SaveSettings({ api_key: apiKey, model });
        statusMsg.textContent = '\u2714 Guardado';
        statusMsg.style.color = '#4caf50';
        setTimeout(() => HideSettingsWindow(), 800);
    } catch (err) {
        statusMsg.textContent = 'Error: ' + String(err);
        statusMsg.style.color = '#e53935';
    }
});

btnCancel.addEventListener('click', () => HideSettingsWindow());
