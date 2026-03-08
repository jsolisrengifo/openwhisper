<script>
  import { onMount } from 'svelte';
  import { TranscribeAudio, PasteText, ShowSettingsWindow, HideWindow, GetSettings } from './bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';

  // Non-reactive internal state
  let mediaRecorder = null;
  let audioChunks = [];
  let isRecording = false;
  let stream = null;

  // Waveform state
  let analyser = null;
  let waveformAnimId = null;
  let waveformBars = $state([0.15, 0.15, 0.15, 0.15, 0.15]);

  // Reactive UI state
  let isConfigured = $state(false);
  let uiState = $state(null);
  let statusMessage = $state('Listo');
  let warnStatus = $state(false);
  let hotkeyDisplay = $state('Ctrl+Space');

  function setState(state, message) {
    uiState = state;
    statusMessage = message ?? 'Listo';
    warnStatus = false;
  }

  function setUnconfigured() {
    statusMessage = '\u2699 Config. pendiente';
    warnStatus = true;
  }

  // ── Waveform animation ───────────────────────────────────────────────────
  function startWaveform(micStream) {
    try {
      const audioCtx = new AudioContext();
      const source = audioCtx.createMediaStreamSource(micStream);
      analyser = audioCtx.createAnalyser();
      analyser.fftSize = 32;
      source.connect(analyser);

      const dataArray = new Uint8Array(analyser.frequencyBinCount);
      const BAR_COUNT = 5;

      function draw() {
        waveformAnimId = requestAnimationFrame(draw);
        analyser.getByteFrequencyData(dataArray);
        const slice = Math.floor(dataArray.length / BAR_COUNT);
        waveformBars = Array.from({ length: BAR_COUNT }, (_, i) => {
          const val = dataArray[i * slice] / 255;
          return Math.max(0.08, val);
        });
      }
      draw();
    } catch (_) {}
  }

  function stopWaveform() {
    if (waveformAnimId) { cancelAnimationFrame(waveformAnimId); waveformAnimId = null; }
    waveformBars = [0.15, 0.15, 0.15, 0.15, 0.15];
    analyser = null;
  }

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
    startWaveform(stream);
  }

  function stopRecording() {
    if (!isRecording || !mediaRecorder) return;
    stopWaveform();
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

  onMount(() => {
    GetSettings().then(s => {
      isConfigured = !!(s.api_key && s.model);
      if (!isConfigured) setUnconfigured();
      if (s.hotkey && s.hotkey.display) hotkeyDisplay = s.hotkey.display;
    }).catch(() => {
      isConfigured = false;
      setUnconfigured();
    });

    Events.On('toggle-recording', toggleRecording);
    Events.On('open-settings', () => ShowSettingsWindow());
  });
</script>

<div class="bar" style="--wails-draggable:drag">
  <button
    class="mic-btn"
    class:recording={uiState === 'recording'}
    class:transcribing={uiState === 'transcribing'}
    class:done={uiState === 'done'}
    class:error={uiState === 'error'}
    title="Grabar ({hotkeyDisplay})"
    style="--wails-draggable:no-drag"
    onclick={toggleRecording}
  >
    {#if uiState === 'recording'}
      <span class="waveform" aria-hidden="true">
        {#each waveformBars as h, i}
          <span class="waveform-bar" style="--h:{h}"></span>
        {/each}
      </span>
    {:else}
      <svg class="mic-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/>
        <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/>
      </svg>
    {/if}
  </button>
  <span class="status-text" class:warn={warnStatus}>{statusMessage}</span>
  <div class="actions" style="--wails-draggable:no-drag">
    <button class="btn-icon btn-settings-toggle" title="Configuracion" onclick={() => ShowSettingsWindow()}>&#9881;</button>
    <button class="btn-icon btn-hide" title="Ocultar" onclick={() => { if (isRecording) stopRecording(); HideWindow(); }}>&#8722;</button>
  </div>
</div>
