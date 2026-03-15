<script>
  import { onMount } from 'svelte';
  import { HideTTSWindow } from '../bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';

  // State
  let ttsState = $state(null); // null | 'processing' | 'playing' | 'paused' | 'done' | 'error'
  let ttsProgress = $state(0);
  let ttsCurrentTime = $state(0);
  let ttsDuration = $state(0);
  let ttsErrorMsg = $state('');
  let ttsAudioEl = null;

  function formatTime(secs) {
    if (!isFinite(secs) || secs < 0) return '0:00';
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${s.toString().padStart(2, '0')}`;
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
    audio.onerror = () => { ttsState = 'error'; ttsErrorMsg = 'Error al reproducir'; };
    audio.play();
    ttsState = 'playing';
  }

  function togglePlay() {
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

  function close() {
    stopTTS();
    HideTTSWindow();
  }

  onMount(() => {
    const cancelProcessing = Events.On('tts:processing', () => {
      stopTTS();
      ttsState = 'processing';
    });

    const cancelAudio = Events.On('tts:audio', (b64) => {
      const mp3 = typeof b64 === 'string' ? b64 : (b64?.data ?? '');
      if (mp3) playTTSAudio(mp3);
    });

    const cancelError = Events.On('tts:error', (msg) => {
      const m = typeof msg === 'string' ? msg : (msg?.data ?? 'Error desconocido');
      stopTTS();
      ttsState = 'error';
      ttsErrorMsg = m.length > 40 ? m.substring(0, 40) + '…' : m;
    });

    return () => {
      cancelProcessing();
      cancelAudio();
      cancelError();
      stopTTS();
    };
  });
</script>

<div class="shell" style="--wails-draggable:drag">
  <!-- Header row -->
  <div class="header" style="--wails-draggable:drag">
    <svg class="vol-icon" class:pulsing={ttsState === 'playing'} viewBox="0 0 24 24" fill="currentColor">
      <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
    </svg>
    <span class="label">TEXTO A VOZ</span>
    <button class="btn-close" onclick={close} title="Cerrar" style="--wails-draggable:no-drag">
      <svg viewBox="0 0 24 24" fill="currentColor">
        <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
      </svg>
    </button>
  </div>

  <!-- Progress bar -->
  <div class="progress-track" role="progressbar" aria-valuenow={ttsProgress} aria-valuemin="0" aria-valuemax="100">
    {#if ttsState === 'processing'}
      <div class="progress-indeterminate"></div>
    {:else}
      <div class="progress-fill" style="width:{ttsProgress}%"></div>
    {/if}
  </div>

  <!-- Controls row -->
  <div class="controls" style="--wails-draggable:no-drag">
    {#if ttsState === 'processing'}
      <div class="spinner"></div>
      <span class="status-text">Sintetizando…</span>
    {:else if ttsState === 'error'}
      <svg class="err-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
      </svg>
      <span class="err-text">{ttsErrorMsg || 'Error'}</span>
    {:else}
      <button class="btn-play" onclick={togglePlay} title={ttsState === 'playing' ? 'Pausar' : 'Reproducir'}>
        {#if ttsState === 'playing'}
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
        {/if}
      </button>
      <span class="time">{formatTime(ttsCurrentTime)} / {formatTime(ttsDuration)}</span>
    {/if}
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

  .shell {
    display: flex;
    flex-direction: column;
    width: 100vw;
    height: 100vh;
    background: #121216;
    border: 1px solid rgba(79, 195, 247, 0.22);
    border-radius: 12px;
    padding: 12px 12px;
    gap: 8px;
    overflow: hidden;
  }

  /* Header */
  .header {
    display: flex;
    align-items: center;
    gap: 7px;
    flex-shrink: 0;
  }

  .vol-icon {
    width: 15px; height: 15px;
    color: #4fc3f7;
    flex-shrink: 0;
    transition: opacity 0.3s;
  }
  .vol-icon.pulsing {
    animation: vol-pulse 1.5s ease-in-out infinite;
  }
  @keyframes vol-pulse { 0%,100%{opacity:1} 50%{opacity:0.45} }

  .label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #81d4fa;
    flex: 1;
  }

  .btn-close {
    background: none; border: none; cursor: pointer;
    color: rgba(255,255,255,0.28);
    padding: 2px; display: flex; align-items: center;
    border-radius: 4px;
    transition: color 0.12s;
  }
  .btn-close svg { width: 13px; height: 13px; }
  .btn-close:hover { color: rgba(255,255,255,0.70); }

  /* Progress */
  .progress-track {
    width: 100%;
    height: 3px;
    background: rgba(255,255,255,0.10);
    border-radius: 2px;
    overflow: hidden;
    flex-shrink: 0;
    position: relative;
  }
  .progress-fill {
    height: 100%;
    background: #4fc3f7;
    border-radius: 2px;
    transition: width 0.15s linear;
  }
  .progress-indeterminate {
    position: absolute;
    height: 100%;
    width: 40%;
    background: #4fc3f7;
    border-radius: 2px;
    animation: indeterminate 1.4s ease-in-out infinite;
  }
  @keyframes indeterminate {
    0%   { left: -40%; }
    100% { left: 100%; }
  }

  /* Controls */
  .controls {
    display: flex;
    align-items: center;
    gap: 9px;
    flex-shrink: 0;
  }

  .btn-play {
    display: flex; align-items: center; justify-content: center;
    width: 28px; height: 28px;
    border-radius: 50%;
    background: #4fc3f7;
    border: none; cursor: pointer;
    color: #0d0d0d;
    flex-shrink: 0;
    transition: background 0.12s, transform 0.1s;
  }
  .btn-play svg { width: 16px; height: 16px; }
  .btn-play:hover { background: #81d4fa; transform: scale(1.07); }

  .time {
    font-size: 11px;
    color: rgba(255,255,255,0.45);
    font-variant-numeric: tabular-nums;
  }

  .spinner {
    width: 22px; height: 22px;
    border: 2px solid rgba(79, 195, 247, 0.20);
    border-top-color: #4fc3f7;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    flex-shrink: 0;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .status-text {
    font-size: 12px;
    color: rgba(255,255,255,0.45);
    font-style: italic;
  }

  .err-icon { width: 16px; height: 16px; color: #ef5350; flex-shrink: 0; }
  .err-text { font-size: 11px; color: #ef9a9a; }
</style>
