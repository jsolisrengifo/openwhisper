<script>
  import { onMount } from 'svelte';
  import { GetSettings, SaveSettings, HideSettingsWindow, GetAPIKeyForProvider } from '../bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';

  let activePage = $state('home');

  // Config state
  let apiKey = $state('');
  let models = $state(['gemini-2.0-flash']);
  let provider = $state('gemini');
  let modelsByProvider = $state({});
  let showKey = $state(false);

  const providerDefaults = {
    gemini: { models: ['gemini-2.0-flash'], placeholder: 'AIza...', modelDesc: 'Modelos en orden de prioridad. Ej: gemini-2.0-flash, gemini-2.5-flash-lite' },
    openrouter: { models: ['openai/gpt-4o-audio-preview'], placeholder: 'sk-or-...', modelDesc: 'Modelos en orden de prioridad. Ej: openai/gpt-4o-audio-preview, google/gemini-2.0-flash-001' },
  };

  async function handleProviderChange(newProvider) {
    // Persist current models for the outgoing provider
    modelsByProvider = { ...modelsByProvider, [provider]: [...models] };
    provider = newProvider;
    // Restore models for the incoming provider
    const stored = modelsByProvider[newProvider];
    models = (stored && stored.length > 0) ? [...stored] : [...(providerDefaults[newProvider]?.models || [''])];
    // Load the stored key for this provider (may be empty if not set yet)
    try {
      apiKey = await GetAPIKeyForProvider(newProvider) || '';
    } catch (_) {
      apiKey = '';
    }
    scheduleAutoSave();
  }

  // Hotkey state (recording)
  let hotkeyDisplay = $state('Ctrl+Space');
  let hotkeyModifiers = $state(0x0002);
  let hotkeyVKey = $state(0x20);
  let capturingHotkey = $state(false);

  // Hotkey state (ask IA)
  let askHotkeyDisplay = $state('Ctrl+Alt+A');
  let askHotkeyModifiers = $state(0x0003);
  let askHotkeyVKey = $state(0x41);
  let capturingAskHotkey = $state(false);

  // Appearance state
  let opacity = $state(100);

  // Profiles state
  let profiles = $state([]);
  let activeProfileID = $state('');
  let editingProfileID = $state(null); // ID of profile being edited (null = none)

  // Auto-save debounce
  let saveTimer = null;

  function buildSavePayload() {
    // Keep the map in sync with the current models before saving
    const cleanModels = models.map(m => m.trim()).filter(m => m.length > 0);
    const updatedMap = { ...modelsByProvider, [provider]: cleanModels };
    return {
      api_key: apiKey.trim(),
      model: cleanModels[0] || '',   // backward compat with Go's legacy Model field
      models: cleanModels,
      provider: provider,
      models_by_provider: updatedMap,
      hotkey: { modifiers: hotkeyModifiers, vkey: hotkeyVKey, display: hotkeyDisplay },
      ask_hotkey: { modifiers: askHotkeyModifiers, vkey: askHotkeyVKey, display: askHotkeyDisplay },
      opacity: opacity,
      profiles: profiles,
      active_profile_id: activeProfileID,
    };
  }

  function scheduleAutoSave() {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(async () => {
      if (!apiKey.trim() && models.every(m => !m.trim())) return;
      try {
        await SaveSettings(buildSavePayload());
      } catch (_) {}
    }, 600);
  }

  async function saveNow() {
    clearTimeout(saveTimer);
    try {
      await SaveSettings(buildSavePayload());
    } catch (_) {}
  }

  onMount(() => {
    function loadSettings() {
      GetSettings().then(s => {
        provider = s.provider || 'gemini';
        modelsByProvider = s.models_by_provider || {};
        apiKey = s.api_key || '';
        // New format: models array; fall back to legacy single-model fields for migration
        if (s.models && s.models.length > 0) {
          models = [...s.models];
        } else if (s.model) {
          models = [s.model];
        } else {
          const stored = modelsByProvider[provider];
          models = (stored && stored.length > 0) ? [...stored] : [...(providerDefaults[provider]?.models || [''])];
        }
        if (s.hotkey && s.hotkey.display) {
          hotkeyDisplay = s.hotkey.display;
          hotkeyModifiers = s.hotkey.modifiers;
          hotkeyVKey = s.hotkey.vkey;
        }
        if (s.ask_hotkey && s.ask_hotkey.display) {
          askHotkeyDisplay = s.ask_hotkey.display;
          askHotkeyModifiers = s.ask_hotkey.modifiers;
          askHotkeyVKey = s.ask_hotkey.vkey;
        }
        opacity = (s.opacity && s.opacity > 0) ? s.opacity : 100;
        profiles = s.profiles && s.profiles.length > 0 ? s.profiles : [];
        activeProfileID = s.active_profile_id || (profiles[0]?.id ?? '');
      }).catch(() => {});
    }

    loadSettings();

    const cancel = Events.On('settings:show', () => loadSettings());
    return () => cancel();
  });

  function handleGlobalKeyDown(e) {
    if (!capturingHotkey && !capturingAskHotkey) return;
    if (['Control', 'Alt', 'Shift', 'Meta'].includes(e.key)) return;
    e.preventDefault();

    let mods = 0;
    const parts = [];
    if (e.ctrlKey)  { mods |= 0x0002; parts.push('Ctrl');  }
    if (e.altKey)   { mods |= 0x0001; parts.push('Alt');   }
    if (e.shiftKey) { mods |= 0x0004; parts.push('Shift'); }
    if (e.metaKey)  { mods |= 0x0008; parts.push('Win');   }

    const keyName = e.key === ' ' ? 'Space' : (e.key.length === 1 ? e.key.toUpperCase() : e.key);
    parts.push(keyName);
    const display = parts.join('+');

    if (capturingHotkey) {
      hotkeyModifiers = mods;
      hotkeyVKey = e.keyCode;
      hotkeyDisplay = display;
      capturingHotkey = false;
    } else {
      askHotkeyModifiers = mods;
      askHotkeyVKey = e.keyCode;
      askHotkeyDisplay = display;
      capturingAskHotkey = false;
    }
    scheduleAutoSave();
  }

  // ── Model list management ───────────────────────────────────────────────
  function addModel() {
    models = [...models, ''];
    scheduleAutoSave();
  }

  function removeModel(i) {
    if (models.length <= 1) return;
    models = models.filter((_, idx) => idx !== i);
    scheduleAutoSave();
  }

  function moveModelUp(i) {
    if (i <= 0) return;
    const arr = [...models];
    [arr[i - 1], arr[i]] = [arr[i], arr[i - 1]];
    models = arr;
    scheduleAutoSave();
  }

  function moveModelDown(i) {
    if (i >= models.length - 1) return;
    const arr = [...models];
    [arr[i], arr[i + 1]] = [arr[i + 1], arr[i]];
    models = arr;
    scheduleAutoSave();
  }

  function updateModel(i, value) {
    const arr = [...models];
    arr[i] = value;
    models = arr;
    scheduleAutoSave();
  }

  // ── Profile management ──────────────────────────────────────────────────
  function addProfile() {
    const id = 'profile_' + Date.now();
    profiles = [...profiles, { id, name: 'Nuevo perfil', prompt: '' }];
    editingProfileID = id;
    activeProfileID = id;
    scheduleAutoSave();
  }

  function deleteProfile(id) {
    if (profiles.length <= 1) return; // keep at least one
    profiles = profiles.filter(p => p.id !== id);
    if (activeProfileID === id) activeProfileID = profiles[0].id;
    if (editingProfileID === id) editingProfileID = null;
    scheduleAutoSave();
  }

  function updateProfileField(id, field, value) {
    profiles = profiles.map(p => p.id === id ? { ...p, [field]: value } : p);
    scheduleAutoSave();
  }
</script>

<svelte:window onkeydown={handleGlobalKeyDown} />

<div class="shell">
  <!-- Sidebar -->
  <nav class="sidebar">
    <div class="brand">
      <svg class="brand-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/>
        <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/>
      </svg>
      <span class="brand-name">OpenWhisper</span>
    </div>

    <ul class="nav-list">
      <li>
        <button class="nav-item" class:active={activePage === 'home'} onclick={() => activePage = 'home'}>
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/></svg>
          <span>Home</span>
        </button>
      </li>
      <li>
        <button class="nav-item" class:active={activePage === 'profiles'} onclick={() => activePage = 'profiles'}>
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M4 6h16v2H4zm0 5h16v2H4zm0 5h16v2H4z"/></svg>
          <span>Perfiles</span>
        </button>
      </li>
      <li>
        <button class="nav-item" class:active={activePage === 'config'} onclick={() => activePage = 'config'}>
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M19.14 12.94c.04-.3.06-.61.06-.94s-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.07.62-.07.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.37 1.04.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.57 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/></svg>
          <span>Configuraci&#243;n</span>
        </button>
      </li>
    </ul>
  </nav>

  <!-- Content -->
  <main class="content">
    {#if activePage === 'home'}
      <div class="page page-home">
        <div class="home-hero">
          <svg class="hero-icon" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/>
            <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/>
          </svg>
          <h1>OpenWhisper</h1>
          <p class="hero-sub">Transcripci&#243;n de voz con inteligencia artificial</p>
        </div>

        <div class="feature-grid">
          <div class="feature-card">
            <span class="feature-icon">&#127897;</span>
            <div>
              <strong>Grabaci&#243;n global</strong>
              <p>Activa el micr&#243;fono desde cualquier aplicaci&#243;n con tu acceso directo.</p>
            </div>
          </div>
          <div class="feature-card">
            <span class="feature-icon">&#9889;</span>
            <div>
              <strong>Transcripci&#243;n instant&#225;nea</strong>
              <p>Usa Gemini AI para convertir tu voz en texto con alta precisi&#243;n.</p>
            </div>
          </div>
          <div class="feature-card">
            <span class="feature-icon">&#128203;</span>
            <div>
              <strong>Pegado autom&#225;tico</strong>
              <p>El texto transcrito se pega donde tengas el cursor.</p>
            </div>
          </div>
          <div class="feature-card">
            <span class="feature-icon">&#128274;</span>
            <div>
              <strong>Privacidad</strong>
              <p>Tu API key se guarda localmente, nunca en servidores externos.</p>
            </div>
          </div>
        </div>
      </div>

    {:else if activePage === 'profiles'}
      <div class="page page-profiles">
        <div class="profiles-header">
          <div>
            <h2 class="profiles-title">Perfiles de dictado</h2>
            <p class="profiles-desc">Cada perfil define el comportamiento de la transcripci&#243;n mediante un prompt personalizado.</p>
          </div>
          <button class="btn-add-profile" onclick={addProfile}>&#43; Nuevo perfil</button>
        </div>

        <div class="profiles-list">
          {#each profiles as profile (profile.id)}
            <div class="profile-card" class:active-profile={profile.id === activeProfileID}>
              <div class="profile-card-top">
                <div class="profile-name-row">
                  <input
                    class="profile-name-input"
                    type="text"
                    value={profile.name}
                    oninput={(e) => updateProfileField(profile.id, 'name', e.target.value)}
                    placeholder="Nombre del perfil"
                  />
                  <div class="profile-actions">
                    {#if profile.id !== activeProfileID}
                      <button
                        class="btn-set-active"
                        onclick={() => { activeProfileID = profile.id; saveNow(); }}
                        title="Usar este perfil"
                      >Activar</button>
                    {:else}
                      <span class="badge-active">Activo</span>
                    {/if}
                    <button
                      class="btn-profile-del"
                      onclick={() => deleteProfile(profile.id)}
                      title="Eliminar perfil"
                      disabled={profiles.length <= 1}
                    >
                      <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/></svg>
                    </button>
                  </div>
                </div>

                <button
                  class="btn-expand"
                  onclick={() => editingProfileID = editingProfileID === profile.id ? null : profile.id}
                >
                  {editingProfileID === profile.id ? '▲ Ocultar prompt' : '▼ Editar prompt'}
                </button>
              </div>

              {#if editingProfileID === profile.id}
                <div class="profile-prompt-wrap">
                  <label class="prompt-label" for="prompt-{profile.id}">Prompt del sistema</label>
                  <textarea
                    id="prompt-{profile.id}"
                    class="profile-prompt"
                    value={profile.prompt}
                    oninput={(e) => updateProfileField(profile.id, 'prompt', e.target.value)}
                    placeholder="Describe c&#243;mo debe procesar el audio. Ej: Transcribe exactamente lo que se dice, en espa&#241;ol..."
                    rows="5"
                  ></textarea>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      </div>

    {:else}
      <div class="page page-config">

        <div class="section-group">
          <p class="group-label">PROVEEDOR DE IA</p>

          <div class="setting-row">
            <div class="setting-info">
              <span class="setting-title">Proveedor</span>
              <span class="setting-desc">Gemini usa tu cuota de Google; OpenRouter permite modelos alternativos</span>
            </div>
            <div class="input-wrap">
              <select
                class="provider-select"
                value={provider}
                onchange={(e) => handleProviderChange(e.target.value)}
              >
                <option value="gemini">Gemini (Google)</option>
                <option value="openrouter">OpenRouter</option>
              </select>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-info">
              <span class="setting-title">API Key</span>
              <span class="setting-desc">{provider === 'openrouter' ? 'Obt\u00e9n tu clave en openrouter.ai' : 'Obt\u00e9n tu clave en Google AI Studio'}</span>
            </div>
            <div class="input-wrap">
              <input
                type={showKey ? 'text' : 'password'}
                bind:value={apiKey}
                oninput={scheduleAutoSave}
                placeholder={providerDefaults[provider]?.placeholder ?? 'API Key...'}
                autocomplete="off"
                spellcheck="false"
              />
              <button class="btn-eye" onclick={() => showKey = !showKey} title={showKey ? 'Ocultar' : 'Mostrar'}>
                {#if showKey}
                  <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78l3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z"/></svg>
                {:else}
                  <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z"/></svg>
                {/if}
              </button>
            </div>
          </div>

          <div class="setting-row setting-row--models">
            <div class="setting-info">
              <span class="setting-title">Modelos</span>
              <span class="setting-desc">{providerDefaults[provider]?.modelDesc ?? 'Nombre del modelo'}</span>
            </div>
            <div class="models-list">
              {#each models as m, i (i)}
                <div class="model-row">
                  <input
                    type="text"
                    class="model-input"
                    value={m}
                    oninput={(e) => updateModel(i, e.target.value)}
                    placeholder={providerDefaults[provider]?.models?.[0] ?? 'modelo'}
                  />
                  <div class="model-row-actions">
                    <button
                      class="btn-model-order"
                      onclick={() => moveModelUp(i)}
                      disabled={i === 0}
                      title="Subir"
                    >&#8593;</button>
                    <button
                      class="btn-model-order"
                      onclick={() => moveModelDown(i)}
                      disabled={i === models.length - 1}
                      title="Bajar"
                    >&#8595;</button>
                    <button
                      class="btn-model-remove"
                      onclick={() => removeModel(i)}
                      disabled={models.length <= 1}
                      title="Eliminar"
                    >&#215;</button>
                  </div>
                </div>
              {/each}
              <button class="btn-add-model" onclick={addModel}>&#43; Agregar modelo</button>
            </div>
          </div>
        </div>

        <div class="section-group">
          <p class="group-label">ACCESOS DIRECTOS</p>

          <div class="setting-row">
            <div class="setting-info">
              <span class="setting-title">Activar grabaci&#243;n</span>
              <span class="setting-desc">Inicia y detiene la grabaci&#243;n</span>
            </div>
            <div class="hotkey-wrap">
              {#if capturingHotkey}
                <div class="hotkey-capture" role="button" tabindex="0"
                  onclick={() => capturingHotkey = false}
                  onkeydown={(e) => e.key === 'Enter' && (capturingHotkey = false)}>
                  <span class="capture-hint">Presiona la combinaci&#243;n&#8230;</span>
                </div>
              {:else}
                <div class="hotkey-keys" role="button" tabindex="0"
                  onclick={() => capturingHotkey = true} title="Clic para cambiar"
                  onkeydown={(e) => e.key === 'Enter' && (capturingHotkey = true)}>
                  {#each hotkeyDisplay.split('+') as part}
                    <kbd>{part}</kbd>
                  {/each}
                </div>
              {/if}
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-info">
              <span class="setting-title">Preguntar a la IA</span>
              <span class="setting-desc">Graba una pregunta y obtén respuesta directa</span>
            </div>
            <div class="hotkey-wrap">
              {#if capturingAskHotkey}
                <div class="hotkey-capture" role="button" tabindex="0"
                  onclick={() => capturingAskHotkey = false}
                  onkeydown={(e) => e.key === 'Enter' && (capturingAskHotkey = false)}>
                  <span class="capture-hint">Presiona la combinaci&#243;n&#8230;</span>
                </div>
              {:else}
                <div class="hotkey-keys" role="button" tabindex="0"
                  onclick={() => capturingAskHotkey = true} title="Clic para cambiar"
                  onkeydown={(e) => e.key === 'Enter' && (capturingAskHotkey = true)}>
                  {#each askHotkeyDisplay.split('+') as part}
                    <kbd>{part}</kbd>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="section-group">
          <p class="group-label">APARIENCIA</p>

          <div class="setting-row">
            <div class="setting-info">
              <span class="setting-title">Transparencia</span>
              <span class="setting-desc">Opacidad de la ventana flotante</span>
            </div>
            <div class="opacity-wrap">
              <input
                type="range"
                min="10"
                max="100"
                step="5"
                bind:value={opacity}
                oninput={scheduleAutoSave}
                class="opacity-slider"
                style="--val: {opacity}"
              />
              <span class="opacity-value">{opacity}%</span>
            </div>
          </div>
        </div>

      </div>
    {/if}
  </main>
</div>

<style>
  /*  Base  */
  :global(*, *::before, *::after) { box-sizing: border-box; margin: 0; padding: 0; }
  :global(html, body) {
    width: 100%; height: 100%; overflow: hidden;
    background: #f5f5f5;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    font-size: 13px; color: #1a1a1a; user-select: none;
  }

  /*  Shell  */
  .shell {
    display: flex;
    width: 100vw; height: 100vh;
    overflow: hidden;
    background: #f5f5f5;
  }

  /*  Sidebar  */
  .sidebar {
    width: 180px;
    min-width: 180px;
    background: #ececec;
    border-right: 1px solid rgba(0,0,0,0.08);
    display: flex;
    flex-direction: column;
    padding: 18px 0 16px;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 0 18px 16px;
    border-bottom: 1px solid rgba(0,0,0,0.07);
    margin-bottom: 10px;
  }
  .brand-icon { width: 20px; height: 20px; color: #0277bd; flex-shrink: 0; }
  .brand-name { font-size: 13px; font-weight: 700; color: #1a1a1a; letter-spacing: -0.2px; }

  .nav-list {
    list-style: none;
    padding: 0 8px;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .nav-item {
    display: flex; align-items: center; gap: 9px;
    width: 100%;
    padding: 7px 10px;
    border: none; border-radius: 6px;
    background: transparent;
    color: rgba(0,0,0,0.45);
    font-size: 12.5px; font-family: inherit;
    cursor: pointer;
    transition: background 0.12s, color 0.12s;
    text-align: left;
  }
  .nav-item svg { width: 15px; height: 15px; flex-shrink: 0; opacity: 0.6; }
  .nav-item:hover { background: rgba(0,0,0,0.06); color: rgba(0,0,0,0.80); }
  .nav-item.active { background: rgba(2,119,189,0.10); color: #0277bd; }
  .nav-item.active svg { opacity: 1; }

  /*  Content  */
  .content {
    flex: 1;
    overflow-y: auto;
    padding: 28px 32px;
    display: flex;
    flex-direction: column;
  }

  .page { display: flex; flex-direction: column; flex: 1; }

  /*  HOME  */
  .page-home {
    justify-content: center; align-items: center; gap: 28px;
  }
  .home-hero {
    display: flex; flex-direction: column; align-items: center; gap: 10px; text-align: center;
  }
  .hero-icon { width: 52px; height: 52px; color: #0277bd; }
  .home-hero h1 { font-size: 24px; font-weight: 700; color: #1a1a1a; letter-spacing: -0.5px; }
  .hero-sub { font-size: 13px; color: rgba(0,0,0,0.40); }

  .feature-grid {
    display: grid; grid-template-columns: 1fr 1fr; gap: 10px; width: 100%;
  }
  .feature-card {
    background: #fff;
    border: 1px solid rgba(0,0,0,0.08);
    border-radius: 10px;
    padding: 14px;
    display: flex; gap: 11px; align-items: flex-start;
  }
  .feature-icon { font-size: 20px; line-height: 1; flex-shrink: 0; margin-top: 1px; }
  .feature-card strong { display: block; font-size: 12px; font-weight: 600; color: rgba(0,0,0,0.80); margin-bottom: 3px; }
  .feature-card p { font-size: 11.5px; color: rgba(0,0,0,0.38); line-height: 1.5; }

  /*  CONFIG  */
  .page-config { gap: 28px; }

  .section-group { display: flex; flex-direction: column; }

  .group-label {
    font-size: 10.5px;
    font-weight: 600;
    color: rgba(0,0,0,0.35);
    letter-spacing: 0.8px;
    padding-bottom: 10px;
    border-bottom: 1px solid rgba(0,0,0,0.08);
    margin-bottom: 0;
  }

  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 14px 0;
    border-bottom: 1px solid rgba(0,0,0,0.06);
  }
  .setting-row:last-child { border-bottom: none; }

  .setting-info {
    display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 0;
  }
  .setting-title { font-size: 13px; font-weight: 500; color: rgba(0,0,0,0.82); }
  .setting-desc { font-size: 11.5px; color: rgba(0,0,0,0.38); }

  /* Inputs */
  .input-wrap {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
    width: 220px;
  }

  input[type="text"],
  input[type="password"],
  select.provider-select {
    flex: 1;
    min-width: 0;
    background: #fff;
    border: 1px solid rgba(0,0,0,0.15);
    border-radius: 7px;
    color: #1a1a1a;
    padding: 7px 10px;
    font-size: 12.5px;
    outline: none;
    font-family: inherit;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  select.provider-select {
    cursor: pointer;
    appearance: auto;
  }
  input:focus,
  select.provider-select:focus {
    border-color: #0277bd;
    box-shadow: 0 0 0 3px rgba(2,119,189,0.12);
  }
  input::placeholder { color: rgba(0,0,0,0.28); }

  .btn-eye {
    background: none; border: none;
    color: rgba(0,0,0,0.35);
    cursor: pointer;
    padding: 3px 2px;
    display: flex; align-items: center;
    flex-shrink: 0;
    transition: color 0.12s;
  }
  .btn-eye svg { width: 16px; height: 16px; }
  .btn-eye:hover { color: rgba(0,0,0,0.70); }

  /* Model list */
  .setting-row--models {
    align-items: flex-start;
  }
  .models-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex-shrink: 0;
    width: 220px;
  }
  .model-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .model-input {
    flex: 1;
    min-width: 0;
  }
  .model-row-actions {
    display: flex;
    gap: 3px;
    flex-shrink: 0;
  }
  .btn-model-order,
  .btn-model-remove {
    background: none;
    border: 1px solid rgba(0,0,0,0.15);
    border-radius: 5px;
    color: rgba(0,0,0,0.50);
    cursor: pointer;
    width: 24px;
    height: 24px;
    font-size: 13px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    transition: background 0.12s, color 0.12s;
  }
  .btn-model-order:hover:not(:disabled),
  .btn-model-remove:hover:not(:disabled) {
    background: rgba(0,0,0,0.07);
    color: rgba(0,0,0,0.80);
  }
  .btn-model-order:disabled,
  .btn-model-remove:disabled {
    opacity: 0.28;
    cursor: default;
  }
  .btn-add-model {
    align-self: flex-start;
    background: none;
    border: 1px dashed rgba(0,0,0,0.22);
    border-radius: 6px;
    color: rgba(0,0,0,0.50);
    cursor: pointer;
    font-size: 11.5px;
    font-family: inherit;
    padding: 4px 10px;
    margin-top: 2px;
    transition: background 0.12s, color 0.12s, border-color 0.12s;
  }
  .btn-add-model:hover {
    background: rgba(2,119,189,0.07);
    color: #0277bd;
    border-color: rgba(2,119,189,0.35);
  }

  /* Hotkey */
  .hotkey-wrap { flex-shrink: 0; }

  .hotkey-keys {
    display: flex; align-items: center; gap: 3px;
    cursor: pointer;
    padding: 4px 6px;
    border-radius: 6px;
    transition: background 0.12s;
  }
  .hotkey-keys:hover { background: rgba(0,0,0,0.06); }

  .hotkey-capture {
    display: flex; align-items: center;
    padding: 6px 12px;
    background: rgba(2,119,189,0.06);
    border: 1px solid rgba(2,119,189,0.30);
    border-radius: 7px;
    cursor: pointer;
    animation: pulse-border 1.2s ease-in-out infinite;
  }

  @keyframes pulse-border {
    0%, 100% { border-color: rgba(2,119,189,0.40); }
    50%       { border-color: rgba(2,119,189,0.12); }
  }

  .capture-hint { font-size: 11.5px; color: #0277bd; font-style: italic; white-space: nowrap; }

  kbd {
    background: #fff;
    border: 1px solid rgba(0,0,0,0.18);
    border-bottom-width: 2px;
    border-radius: 4px;
    padding: 3px 8px;
    font-size: 11.5px;
    font-family: inherit;
    color: rgba(0,0,0,0.70);
    white-space: nowrap;
    box-shadow: 0 1px 0 rgba(0,0,0,0.08);
  }

  /* Opacity slider */
  .opacity-wrap {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
    width: 220px;
  }

  .opacity-slider {
    flex: 1;
    -webkit-appearance: none;
    appearance: none;
    height: 4px;
    border-radius: 2px;
    background: linear-gradient(to right, #0277bd calc(var(--val, 100) * 1%), rgba(0,0,0,0.15) calc(var(--val, 100) * 1%));
    outline: none;
    cursor: pointer;
  }
  .opacity-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #0277bd;
    cursor: pointer;
    border: 2px solid #fff;
    box-shadow: 0 1px 3px rgba(0,0,0,0.25);
    transition: box-shadow 0.15s;
  }
  .opacity-slider::-webkit-slider-thumb:hover {
    box-shadow: 0 0 0 4px rgba(2,119,189,0.18);
  }

  .opacity-value {
    font-size: 12px;
    font-weight: 600;
    color: rgba(0,0,0,0.55);
    min-width: 34px;
    text-align: right;
  }

  /* Profiles page */
  .page-profiles { gap: 20px; overflow-y: auto; }

  .profiles-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    flex-shrink: 0;
  }
  .profiles-title { font-size: 15px; font-weight: 700; color: #1a1a1a; margin-bottom: 4px; }
  .profiles-desc { font-size: 11.5px; color: rgba(0,0,0,0.40); line-height: 1.5; max-width: 320px; }

  .btn-add-profile {
    background: #0277bd;
    color: #fff;
    border: none;
    border-radius: 7px;
    padding: 7px 14px;
    font-size: 12px;
    font-family: inherit;
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
    transition: background 0.12s;
  }
  .btn-add-profile:hover { background: #0288d1; }

  .profiles-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    flex: 1;
    overflow-y: auto;
  }

  .profile-card {
    background: #fff;
    border: 1px solid rgba(0,0,0,0.10);
    border-radius: 10px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    transition: border-color 0.12s;
  }
  .profile-card.active-profile {
    border-color: rgba(2,119,189,0.45);
    background: rgba(2,119,189,0.03);
  }

  .profile-card-top { display: flex; flex-direction: column; gap: 8px; }

  .profile-name-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .profile-name-input {
    flex: 1;
    min-width: 0;
    background: transparent;
    border: none;
    border-bottom: 1px solid rgba(0,0,0,0.12);
    border-radius: 0;
    color: #1a1a1a;
    padding: 4px 2px;
    font-size: 13px;
    font-weight: 600;
    font-family: inherit;
    outline: none;
    transition: border-color 0.15s;
  }
  .profile-name-input:focus { border-bottom-color: #0277bd; }

  .profile-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .btn-set-active {
    background: none;
    border: 1px solid rgba(2,119,189,0.40);
    color: #0277bd;
    border-radius: 6px;
    padding: 4px 10px;
    font-size: 11.5px;
    font-family: inherit;
    cursor: pointer;
    transition: background 0.12s, color 0.12s;
  }
  .btn-set-active:hover { background: rgba(2,119,189,0.08); }

  .badge-active {
    font-size: 11px;
    font-weight: 600;
    color: #0277bd;
    background: rgba(2,119,189,0.10);
    border-radius: 4px;
    padding: 3px 8px;
  }

  .btn-profile-del {
    background: none;
    border: none;
    color: rgba(0,0,0,0.28);
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    border-radius: 5px;
    transition: color 0.12s, background 0.12s;
  }
  .btn-profile-del svg { width: 15px; height: 15px; }
  .btn-profile-del:hover:not(:disabled) { color: #d32f2f; background: rgba(211,47,47,0.06); }
  .btn-profile-del:disabled { opacity: 0.25; cursor: not-allowed; }

  .btn-expand {
    background: none;
    border: none;
    color: rgba(0,0,0,0.40);
    font-size: 11.5px;
    font-family: inherit;
    cursor: pointer;
    padding: 2px 0;
    text-align: left;
    transition: color 0.12s;
  }
  .btn-expand:hover { color: rgba(0,0,0,0.70); }

  .profile-prompt-wrap {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .prompt-label {
    font-size: 10.5px;
    font-weight: 600;
    color: rgba(0,0,0,0.35);
    letter-spacing: 0.5px;
  }

  .profile-prompt {
    width: 100%;
    background: #f9f9f9;
    border: 1px solid rgba(0,0,0,0.12);
    border-radius: 7px;
    color: #1a1a1a;
    padding: 9px 11px;
    font-size: 12px;
    font-family: inherit;
    line-height: 1.55;
    resize: vertical;
    outline: none;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  .profile-prompt:focus {
    border-color: #0277bd;
    box-shadow: 0 0 0 3px rgba(2,119,189,0.10);
  }
</style>
