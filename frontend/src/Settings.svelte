<script>
  import { onMount } from 'svelte';
  import { GetSettings, SaveSettings, HideSettingsWindow } from './bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';

  let apiKey = $state('');
  let model = $state('gemini-2.0-flash');
  let showKey = $state(false);
  let statusMsg = $state('');
  let statusColor = $state('rgba(255,255,255,0.45)');
  let apiKeyBorderColor = $state('');
  let modelBorderColor = $state('');

  onMount(() => {
    function loadSettings() {
      GetSettings().then(s => {
        apiKey = s.api_key || '';
        model = s.model || 'gemini-2.0-flash';
      }).catch(() => {});
    }

    loadSettings();

    // Re-load settings each time Go calls ShowSettingsWindow()
    const cancel = Events.On('settings:show', () => {
      statusMsg = '';
      statusColor = 'rgba(255,255,255,0.45)';
      loadSettings();
    });

    return () => cancel();
  });

  async function save() {
    statusMsg = '';
    apiKeyBorderColor = '';
    modelBorderColor = '';

    if (!apiKey.trim()) {
      apiKeyBorderColor = '#e53935';
      setTimeout(() => { apiKeyBorderColor = ''; }, 2000);
      return;
    }
    if (!model.trim()) {
      modelBorderColor = '#e53935';
      setTimeout(() => { modelBorderColor = ''; }, 2000);
      return;
    }

    try {
      await SaveSettings({ api_key: apiKey.trim(), model: model.trim() });
      statusMsg = '\u2714 Guardado';
      statusColor = '#4caf50';
      setTimeout(() => HideSettingsWindow(), 800);
    } catch (err) {
      statusMsg = 'Error: ' + String(err);
      statusColor = '#e53935';
    }
  }
</script>

<h2>&#9881; Configuración</h2>

<div class="field">
  <label for="api-key">API Key (Gemini)</label>
  <div class="input-row">
    <input
      id="api-key"
      type={showKey ? 'text' : 'password'}
      bind:value={apiKey}
      placeholder="Pega tu API key aquí"
      autocomplete="off"
      style:border-color={apiKeyBorderColor || undefined}
    />
    <button class="btn-icon" title="Mostrar/ocultar" onclick={() => { showKey = !showKey; }}>&#128065;</button>
  </div>
</div>

<div class="field">
  <label for="model-input">Modelo</label>
  <input
    id="model-input"
    type="text"
    bind:value={model}
    placeholder="gemini-2.0-flash"
    style:border-color={modelBorderColor || undefined}
  />
</div>

<div class="btn-row">
  <button class="btn-primary" onclick={save}>Guardar</button>
  <button class="btn-secondary" onclick={() => HideSettingsWindow()}>Cancelar</button>
</div>

<p class="status-msg" style:color={statusColor}>{statusMsg}</p>

<style>
  h2 {
    font-size: 15px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.85);
    margin-bottom: 4px;
  }

  .status-msg {
    font-size: 12px;
    min-height: 16px;
  }
</style>
