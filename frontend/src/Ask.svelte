<script>
  import { onMount, tick } from 'svelte';
  import {
    HideAskWindow, PasteText, CopyText, RegenerateAsk, AskFollowUp
  } from '../bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';

  // ── State ────────────────────────────────────────────────────────────────
  /** @type {{ role: 'user'|'model', text: string, isAudio?: boolean }[]} */
  let chatHistory = $state([]);
  let isRegenerating = $state(false);
  let isCopied = $state(false);
  let isFollowUpRecording = $state(false);
  let isFollowUpProcessing = $state(false);

  // TTS state
  let ttsState = $state(null); // null | 'processing' | 'playing' | 'paused' | 'done' | 'error'
  let ttsProgress = $state(0);
  let ttsCurrentTime = $state(0);
  let ttsDuration = $state(0);
  let ttsErrorMsg = $state('');
  let ttsAudioEl = null;

  // DOM refs
  let chatBodyEl = $state(null);

  // Non-reactive (mediarecorder state)
  let followUpRecorder = null;
  let followUpChunks = [];
  let followUpStream = null;

  // Derived: last model answer text
  function lastAnswerText() {
    for (let i = chatHistory.length - 1; i >= 0; i--) {
      if (chatHistory[i].role === 'model') return chatHistory[i].text;
    }
    return '';
  }

  async function scrollToBottom() {
    await tick();
    if (chatBodyEl) chatBodyEl.scrollTop = chatBodyEl.scrollHeight;
  }

  // ── Event listeners ──────────────────────────────────────────────────────
  onMount(() => {
    const cancelResponse = Events.On('ask:response', (text) => {
      const t = typeof text === 'string' ? text : (text?.data ?? '');
      chatHistory = [...chatHistory, { role: 'model', text: t }];
      scrollToBottom();
    });

    const cancelNewChat = Events.On('ask:new-chat', () => {
      chatHistory = [];
      isCopied = false;
      isRegenerating = false;
      stopTTS();
    });

    const cancelTTSProcessing = Events.On('tts:processing', () => {
      stopTTS();
      ttsState = 'processing';
    });

    const cancelTTSAudio = Events.On('tts:audio', (b64) => {
      const mp3 = typeof b64 === 'string' ? b64 : (b64?.data ?? '');
      if (mp3) playTTSAudio(mp3);
    });

    const cancelTTSError = Events.On('tts:error', (msg) => {
      const m = typeof msg === 'string' ? msg : (msg?.data ?? 'Error desconocido');
      stopTTS();
      ttsState = 'error';
      ttsErrorMsg = m;
    });

    function onKeyDown(e) {
      if (e.key === 'Escape' && !isFollowUpRecording) close();
    }
    window.addEventListener('keydown', onKeyDown);

    return () => {
      cancelResponse();
      cancelNewChat();
      cancelTTSProcessing();
      cancelTTSAudio();
      cancelTTSError();
      window.removeEventListener('keydown', onKeyDown);
      stopFollowUp();
      stopTTS();
    };
  });

  // ── Actions ──────────────────────────────────────────────────────────────
  function close() {
    chatHistory = [];
    isCopied = false;
    stopFollowUp();
    stopTTS();
    HideAskWindow();
  }

  async function insertAtCursor() {
    const text = lastAnswerText();
    if (!text) return;
    HideAskWindow();
    await new Promise(r => setTimeout(r, 200));
    await PasteText(text);
    chatHistory = [];
  }

  async function copyAnswer() {
    const text = lastAnswerText();
    if (!text) return;
    await CopyText(text);
    isCopied = true;
    setTimeout(() => { isCopied = false; }, 1500);
  }

  async function regenerate() {
    if (isRegenerating) return;
    isRegenerating = true;
    try {
      const result = await RegenerateAsk();
      if (result && result.trim()) {
        // Replace the last model turn if any, otherwise append
        const idx = [...chatHistory].reverse().findIndex(t => t.role === 'model');
        if (idx !== -1) {
          const realIdx = chatHistory.length - 1 - idx;
          chatHistory = [
            ...chatHistory.slice(0, realIdx),
            { role: 'model', text: result.trim() },
            ...chatHistory.slice(realIdx + 1),
          ];
        } else {
          chatHistory = [...chatHistory, { role: 'model', text: result.trim() }];
        }
        scrollToBottom();
      }
    } catch (err) {
      // keep existing answer on error
    } finally {
      isRegenerating = false;
    }
  }

  // ── Follow-up recording ──────────────────────────────────────────────────
  async function toggleFollowUp() {
    if (isFollowUpRecording) {
      stopFollowUp();
    } else {
      await startFollowUp();
    }
  }

  async function startFollowUp() {
    if (isFollowUpProcessing) return;
    try {
      followUpStream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    } catch {
      return;
    }
    followUpChunks = [];
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
      ? 'audio/webm;codecs=opus' : 'audio/webm';
    followUpRecorder = new MediaRecorder(followUpStream, { mimeType });
    followUpRecorder.ondataavailable = e => { if (e.data?.size > 0) followUpChunks.push(e.data); };
    followUpRecorder.onstop = handleFollowUpStop;
    followUpRecorder.start();
    isFollowUpRecording = true;
  }

  function stopFollowUp() {
    if (followUpRecorder && followUpRecorder.state !== 'inactive') {
      followUpRecorder.onstop = handleFollowUpStop;
      followUpRecorder.stop();
    }
    if (followUpStream) { followUpStream.getTracks().forEach(t => t.stop()); followUpStream = null; }
    isFollowUpRecording = false;
  }

  async function handleFollowUpStop() {
    if (isFollowUpProcessing || followUpChunks.length === 0) { followUpChunks = []; return; }
    isFollowUpProcessing = true;
    const blob = new Blob(followUpChunks, { type: 'audio/webm' });
    followUpChunks = [];
    const mimeType = blob.type || 'audio/webm';

    // Build history in Gemini roles format (text-only turns)
    const historyForApi = chatHistory
      .filter(t => !t.isAudio)
      .map(t => ({ role: t.role, text: t.text }));

    // Show user placeholder turn immediately
    chatHistory = [...chatHistory, { role: 'user', text: '🎤 pregunta de seguimiento', isAudio: true }];
    scrollToBottom();

    try {
      const base64 = await blobToBase64(blob);
      const answer = await AskFollowUp(base64, mimeType, historyForApi);
      if (answer && answer.trim()) {
        chatHistory = [...chatHistory, { role: 'model', text: answer.trim() }];
        scrollToBottom();
      }
    } catch {
      // remove the placeholder on error
      chatHistory = chatHistory.filter((_, i) => i !== chatHistory.length - 1);
    } finally {
      isFollowUpProcessing = false;
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

  // ── TTS Playback ─────────────────────────────────────────────────────────
  function formatTime(secs) {
    if (!isFinite(secs)) return '0:00';
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  function playTTSAudio(base64mp3) {
    stopTTS();
    const audio = new Audio('data:audio/mp3;base64,' + base64mp3);
    ttsAudioEl = audio;
    audio.ontimeupdate = () => {
      ttsCurrentTime = audio.currentTime;
      ttsDuration = audio.duration || 0;
      ttsProgress = ttsDuration > 0 ? (ttsCurrentTime / ttsDuration) * 100 : 0;
    };
    audio.onended = () => { ttsState = 'done'; ttsProgress = 100; };
    audio.onerror = () => { ttsState = 'error'; ttsErrorMsg = 'Error al reproducir el audio'; };
    audio.play();
    ttsState = 'playing';
  }

  function toggleTTSPlaying() {
    if (!ttsAudioEl) return;
    if (ttsState === 'playing') {
      ttsAudioEl.pause();
      ttsState = 'paused';
    } else if (ttsState === 'paused') {
      ttsAudioEl.play();
      ttsState = 'playing';
    } else if (ttsState === 'done') {
      ttsAudioEl.currentTime = 0;
      ttsAudioEl.play();
      ttsState = 'playing';
    }
  }

  function stopTTS() {
    if (ttsAudioEl) {
      ttsAudioEl.pause();
      ttsAudioEl.ontimeupdate = null;
      ttsAudioEl.onended = null;
      ttsAudioEl.onerror = null;
      ttsAudioEl = null;
    }
    ttsState = null;
    ttsProgress = 0;
    ttsCurrentTime = 0;
    ttsDuration = 0;
    ttsErrorMsg = '';
  }
</script>

<div class="ask-shell" style="--wails-draggable:drag">
  <!-- Header -->
  <div class="ask-header" style="--wails-draggable:drag">
    <div class="ask-title">
      <svg class="ai-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 14H9V8h2v8zm4 0h-2V8h2v8z"/>
        <path d="M13 2.05v2.02c3.95.49 7 3.85 7 7.93s-3.05 7.44-7 7.93v2.02c5.05-.5 9-4.76 9-9.95s-3.95-9.45-9-9.95z"/>
        <path d="M11 2.05C5.95 2.55 2 6.81 2 12s3.95 9.45 9 9.95v-2.02C7.05 19.44 4 16.08 4 12s3.05-7.44 7-7.93V2.05z"/>
      </svg>
      <span>{chatHistory.length > 2 ? 'Chat IA' : 'Respuesta IA'}</span>
    </div>
    <button class="btn-close" onclick={close} title="Cerrar (Esc)" style="--wails-draggable:no-drag">
      <svg viewBox="0 0 24 24" fill="currentColor">
        <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
      </svg>
    </button>
  </div>

  <!-- Chat body -->
  <div class="ask-body" bind:this={chatBodyEl} style="--wails-draggable:no-drag">
    {#if ttsState === 'processing'}
      <div class="tts-card">
        <div class="tts-spinner"></div>
        <span class="tts-status">Sintetizando audio…</span>
      </div>
    {:else if ttsState === 'playing' || ttsState === 'paused' || ttsState === 'done'}
      <div class="tts-card">
        <div class="tts-card-header">
          <svg viewBox="0 0 24 24" fill="currentColor" class="tts-icon" class:tts-playing={ttsState === 'playing'}>
            <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
          </svg>
          <span class="tts-label">Texto a Voz</span>
          <button class="btn-tts-close" onclick={stopTTS} title="Detener">
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>
          </button>
        </div>
        <div class="tts-progress-wrap" role="progressbar" aria-valuenow={ttsProgress} aria-valuemin="0" aria-valuemax="100">
          <div class="tts-progress-track">
            <div class="tts-progress-fill" style="width:{ttsProgress}%"></div>
          </div>
        </div>
        <div class="tts-controls">
          <button class="btn-tts-play" onclick={toggleTTSPlaying} title={ttsState === 'playing' ? 'Pausar' : 'Reproducir'}>
            {#if ttsState === 'playing'}
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
            {:else}
              <svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
            {/if}
          </button>
          <span class="tts-time">{formatTime(ttsCurrentTime)} / {formatTime(ttsDuration)}</span>
        </div>
      </div>
    {:else if ttsState === 'error'}
      <div class="tts-card tts-card-error">
        <svg viewBox="0 0 24 24" fill="currentColor" class="tts-err-icon"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg>
        <span class="tts-err-msg">{ttsErrorMsg}</span>
      </div>
    {:else if chatHistory.length === 0}
      <p class="placeholder">Esperando respuesta…</p>
    {:else}
      {#each chatHistory as turn}
        {#if turn.role === 'user'}
          <div class="turn turn-user">
            <span class="turn-label">Tú</span>
            <p class="turn-text user-text">{turn.text}</p>
          </div>
        {:else}
          <div class="turn turn-model">
            <span class="turn-label">IA</span>
            <p class="turn-text model-text">{turn.text}</p>
          </div>
        {/if}
      {/each}
      {#if isRegenerating}
        <p class="placeholder thinking">Regenerando…</p>
      {/if}
      {#if isFollowUpProcessing}
        <p class="placeholder thinking">Consultando IA…</p>
      {/if}
    {/if}
  </div>

  <!-- Footer actions -->
  <div class="ask-footer" style="--wails-draggable:no-drag">
    <!-- Quick actions (only shown when there is at least one model answer) -->
    {#if chatHistory.some(t => t.role === 'model')}
      <div class="actions-row">
        <button class="btn-action btn-insert" onclick={insertAtCursor} title="Pegar en el cursor">
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M19 2h-4.18C14.4.84 13.3 0 12 0c-1.3 0-2.4.84-2.82 2H5c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-7 0c.55 0 1 .45 1 1s-.45 1-1 1-1-.45-1-1 .45-1 1-1zm7 18H5V4h2v3h10V4h2v16z"/></svg>
          Insertar
        </button>
        <button class="btn-action btn-copy" onclick={copyAnswer} title="Copiar respuesta">
          {#if isCopied}
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>
            ¡Copiado!
          {:else}
            <svg viewBox="0 0 24 24" fill="currentColor"><path d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"/></svg>
            Copiar
          {/if}
        </button>
        <button class="btn-action btn-regen" onclick={regenerate} disabled={isRegenerating} title="Regenerar respuesta">
          <svg viewBox="0 0 24 24" fill="currentColor" class:spinning={isRegenerating}><path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/></svg>
          {isRegenerating ? '…' : 'Regenerar'}
        </button>
      </div>
    {/if}

    <!-- Follow-up recording + dismiss row -->
    <div class="bottom-row">
      <!-- Follow-up mic button -->
      <button
        class="btn-followup"
        class:recording={isFollowUpRecording}
        onclick={toggleFollowUp}
        disabled={isFollowUpProcessing}
        title={isFollowUpRecording ? 'Detener seguimiento' : 'Preguntar de seguimiento'}
      >
        {#if isFollowUpRecording}
          <svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/><path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/></svg>
        {/if}
        {isFollowUpRecording ? 'Detener' : 'Seguimiento'}
      </button>

      <div class="spacer"></div>

      <button class="btn-dismiss" onclick={close}>Cerrar</button>
    </div>
  </div>
</div>

<style>
  :global(*, *::before, *::after) { box-sizing: border-box; margin: 0; padding: 0; }
  :global(html, body) {
    width: 100%; height: 100%; overflow: hidden;
    background: transparent;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    font-size: 13px;
    color: #f0f0f0;
  }

  .ask-shell {
    display: flex;
    flex-direction: column;
    width: 100vw;
    height: 100vh;
    background: rgba(18, 18, 18, 0.97);
    border: 1px solid rgba(255, 255, 255, 0.10);
    border-radius: 14px;
    overflow: hidden;
  }

  /* ── Header ── */
  .ask-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px 10px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.07);
    flex-shrink: 0;
  }

  .ask-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 600;
    color: rgba(255,255,255,0.85);
    letter-spacing: -0.1px;
  }

  .ai-icon { width: 16px; height: 16px; color: #42a5f5; flex-shrink: 0; }

  .btn-close {
    background: none; border: none;
    color: rgba(255,255,255,0.35); cursor: pointer;
    padding: 4px; display: flex; align-items: center;
    border-radius: 6px; transition: color 0.12s, background 0.12s;
  }
  .btn-close svg { width: 16px; height: 16px; }
  .btn-close:hover { color: rgba(255,255,255,0.80); background: rgba(255,255,255,0.08); }

  /* ── Body / chat ── */
  .ask-body {
    flex: 1;
    overflow-y: auto;
    padding: 12px 16px;
    scrollbar-width: thin;
    scrollbar-color: rgba(255,255,255,0.15) transparent;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .turn { display: flex; flex-direction: column; gap: 2px; }
  .turn-label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    opacity: 0.45;
    padding: 0 2px;
  }
  .turn-user  .turn-label { color: #90caf9; }
  .turn-model .turn-label { color: #80cbc4; }

  .turn-text {
    font-size: 13px;
    line-height: 1.65;
    white-space: pre-wrap;
    word-break: break-word;
    padding: 8px 12px;
    border-radius: 10px;
  }
  .user-text {
    background: rgba(66, 165, 245, 0.10);
    color: rgba(255,255,255,0.75);
    font-style: italic;
    border: 1px solid rgba(66, 165, 245, 0.18);
    align-self: flex-start;
    max-width: 92%;
  }
  .model-text {
    background: rgba(255,255,255,0.05);
    color: rgba(255,255,255,0.88);
    border: 1px solid rgba(255,255,255,0.07);
    user-select: text;
    -webkit-user-select: text;
    max-width: 98%;
  }

  .placeholder {
    font-size: 13px; color: rgba(255,255,255,0.30); font-style: italic;
  }
  .thinking { color: rgba(255,255,255,0.40); animation: pulse 1.2s infinite; }
  @keyframes pulse { 0%,100%{opacity:.4} 50%{opacity:.9} }

  /* ── Footer ── */
  .ask-footer {
    padding: 8px 12px 12px;
    border-top: 1px solid rgba(255,255,255,0.07);
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .actions-row {
    display: flex;
    gap: 6px;
  }

  .btn-action {
    display: flex; align-items: center; gap: 5px;
    font-size: 12px; font-family: inherit;
    padding: 5px 12px; border-radius: 7px; cursor: pointer;
    border: 1px solid rgba(255,255,255,0.12);
    transition: background 0.12s, color 0.12s, border-color 0.12s;
    white-space: nowrap;
  }
  .btn-action svg { width: 14px; height: 14px; flex-shrink: 0; }
  .btn-action:disabled { opacity: 0.45; cursor: default; }

  .btn-insert {
    background: rgba(66,165,245,0.15); color: #90caf9;
    border-color: rgba(66,165,245,0.30);
  }
  .btn-insert:hover { background: rgba(66,165,245,0.25); }

  .btn-copy {
    background: rgba(255,255,255,0.07); color: rgba(255,255,255,0.70);
  }
  .btn-copy:hover { background: rgba(255,255,255,0.14); color: rgba(255,255,255,0.90); }

  .btn-regen {
    background: rgba(255,183,77,0.10); color: #ffcc80;
    border-color: rgba(255,183,77,0.25);
  }
  .btn-regen:hover:not(:disabled) { background: rgba(255,183,77,0.20); }

  @keyframes spin { to { transform: rotate(360deg); } }
  .spinning { animation: spin 0.8s linear infinite; }

  .bottom-row {
    display: flex; align-items: center; gap: 8px;
  }
  .spacer { flex: 1; }

  .btn-followup {
    display: flex; align-items: center; gap: 5px;
    font-size: 12px; font-family: inherit;
    padding: 5px 10px; border-radius: 7px; cursor: pointer;
    background: rgba(255,255,255,0.06);
    border: 1px solid rgba(255,255,255,0.12);
    color: rgba(255,255,255,0.60);
    transition: background 0.12s, color 0.12s;
  }
  .btn-followup svg { width: 13px; height: 13px; }
  .btn-followup:hover:not(:disabled) { background: rgba(255,255,255,0.12); color: rgba(255,255,255,0.85); }
  .btn-followup.recording {
    background: rgba(239,83,80,0.18); color: #ef5350;
    border-color: rgba(239,83,80,0.40);
    animation: pulse 1.2s infinite;
  }
  .btn-followup:disabled { opacity: 0.4; cursor: default; }

  .btn-dismiss {
    background: rgba(255,255,255,0.08);
    border: 1px solid rgba(255,255,255,0.12);
    border-radius: 7px;
    color: rgba(255,255,255,0.65);
    font-size: 12px; font-family: inherit;
    padding: 5px 14px; cursor: pointer;
    transition: background 0.12s, color 0.12s;
  }
  .btn-dismiss:hover { background: rgba(255,255,255,0.14); color: rgba(255,255,255,0.90); }

  /* ── TTS Player ── */
  .tts-card {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    background: rgba(129, 212, 250, 0.07);
    border: 1px solid rgba(129, 212, 250, 0.18);
    border-radius: 12px;
  }

  .tts-card-header {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .tts-icon {
    width: 18px; height: 18px;
    color: #4fc3f7;
    flex-shrink: 0;
  }
  .tts-icon.tts-playing {
    animation: tts-pulse 1.5s ease-in-out infinite;
  }
  @keyframes tts-pulse { 0%,100%{opacity:1} 50%{opacity:0.55} }

  .tts-label {
    font-size: 12px;
    font-weight: 600;
    color: #81d4fa;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    flex: 1;
  }

  .btn-tts-close {
    background: none; border: none; cursor: pointer;
    color: rgba(255,255,255,0.30); padding: 2px;
    display: flex; align-items: center; border-radius: 4px;
    transition: color 0.12s;
  }
  .btn-tts-close svg { width: 14px; height: 14px; }
  .btn-tts-close:hover { color: rgba(255,255,255,0.70); }

  .tts-progress-wrap { width: 100%; }
  .tts-progress-track {
    width: 100%;
    height: 4px;
    background: rgba(255,255,255,0.12);
    border-radius: 2px;
    overflow: hidden;
  }
  .tts-progress-fill {
    height: 100%;
    background: #4fc3f7;
    border-radius: 2px;
    transition: width 0.15s linear;
  }

  .tts-controls {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .btn-tts-play {
    display: flex; align-items: center; justify-content: center;
    width: 34px; height: 34px;
    border-radius: 50%;
    background: #4fc3f7;
    border: none; cursor: pointer;
    color: #0d0d0d;
    transition: background 0.12s, transform 0.1s;
    flex-shrink: 0;
  }
  .btn-tts-play svg { width: 18px; height: 18px; }
  .btn-tts-play:hover { background: #81d4fa; transform: scale(1.06); }

  .tts-time {
    font-size: 11px;
    color: rgba(255,255,255,0.45);
    font-variant-numeric: tabular-nums;
  }

  .tts-status {
    font-size: 13px;
    color: rgba(255,255,255,0.50);
    font-style: italic;
    text-align: center;
  }

  .tts-spinner {
    width: 28px; height: 28px;
    border: 3px solid rgba(79, 195, 247, 0.2);
    border-top-color: #4fc3f7;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin: 0 auto;
  }

  .tts-card-error {
    background: rgba(239, 83, 80, 0.07);
    border-color: rgba(239, 83, 80, 0.20);
    flex-direction: row;
    align-items: center;
  }
  .tts-err-icon { width: 16px; height: 16px; color: #ef5350; flex-shrink: 0; }
  .tts-err-msg { font-size: 12px; color: #ef9a9a; }
</style>
