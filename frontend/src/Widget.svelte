<script>
  import { onMount, tick } from 'svelte';
  import { TranscribeAudio, PasteText, ShowSettingsWindow, HideWindow, GetSettings } from './bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';

  // Non-reactive internal state
  let mediaRecorder = null;
  let audioChunks = [];
  let isRecording = false;
  let isStarting = false;  // guard síncrono para la race condition en startRecording async
  let isProcessing = false; // guard: evita doble transcripción/pegado
  let stream = null;

  // Waveform state
  let analyser = null;
  let waveformAnimId = null;
  let waveformCanvas = $state(null); // bound via bind:this

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
    const canvas = waveformCanvas;
    if (!canvas) return;
    try {
      const audioCtx = new AudioContext();
      const source = audioCtx.createMediaStreamSource(micStream);
      analyser = audioCtx.createAnalyser();
      analyser.fftSize = 1024;
      analyser.smoothingTimeConstant = 0.6;
      source.connect(analyser);

      const bufLen = analyser.fftSize;
      const floatBuf = new Float32Array(bufLen);
      const ctx2d = canvas.getContext('2d');

      const W = canvas.width;   // 38
      const H = canvas.height;  // 28
      const mid = H / 2;

      // Parámetros de barras
      const BAR_W   = 3;     // ancho de barra en px
      const GAP     = 2;     // separación entre barras en px
      const STEP    = BAR_W + GAP;
      const N_BARS  = Math.floor(W / STEP); // ~19 barras
      const BAR_MAX = mid - 2;              // altura máxima (mitad del canvas - margen)
      const BAR_MIN = 1.5;                  // altura mínima visible en silencio

      // Historial de amplitudes (una por barra)
      const amp     = new Float32Array(N_BARS);
      const SAMPLE_MS = 80;  // ~12 barras/segundo → scroll visible pero no tan rápido
      let lastTs = -SAMPLE_MS;

      function draw(ts) {
        waveformAnimId = requestAnimationFrame(draw);

        if (ts - lastTs >= SAMPLE_MS) {
          lastTs = ts;
          analyser.getFloatTimeDomainData(floatBuf);
          // RMS del frame actual
          let sum = 0;
          for (let i = 0; i < bufLen; i++) sum += floatBuf[i] * floatBuf[i];
          const rms = Math.sqrt(sum / bufLen);
          // Scroll: shift left, nuevo valor a la derecha
          amp.copyWithin(0, 1);
          amp[N_BARS - 1] = Math.min(1, rms * 6);
        }

        ctx2d.clearRect(0, 0, W, H);

        for (let i = 0; i < N_BARS; i++) {
          const halfH = amp[i] * BAR_MAX + BAR_MIN;
          const x     = i * STEP;
          const y     = mid - halfH;
          const barH  = halfH * 2;

          // Barra sólida con bordes redondeados, simétrica al centro
          ctx2d.fillStyle = '#ef5350';
          roundRect(ctx2d, x, y, BAR_W, barH, BAR_W / 2);
        }
      }

      requestAnimationFrame(draw);
    } catch (_) {}
  }

  // Dibuja un rectángulo con esquinas redondeadas (compatible con todos los navegadores)
  function roundRect(ctx2d, x, y, w, h, r) {
    if (h < r * 2) r = h / 2;
    ctx2d.beginPath();
    ctx2d.moveTo(x + r, y);
    ctx2d.lineTo(x + w - r, y);
    ctx2d.quadraticCurveTo(x + w, y, x + w, y + r);
    ctx2d.lineTo(x + w, y + h - r);
    ctx2d.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
    ctx2d.lineTo(x + r, y + h);
    ctx2d.quadraticCurveTo(x, y + h, x, y + h - r);
    ctx2d.lineTo(x, y + r);
    ctx2d.quadraticCurveTo(x, y, x + r, y);
    ctx2d.closePath();
    ctx2d.fill();
  }

  function stopWaveform() {
    if (waveformAnimId) { cancelAnimationFrame(waveformAnimId); waveformAnimId = null; }
    if (waveformCanvas) {
      const ctx2d = waveformCanvas.getContext('2d');
      ctx2d.clearRect(0, 0, waveformCanvas.width, waveformCanvas.height);
    }
    analyser = null;
  }

  async function startRecording() {
    if (isRecording || isStarting || isProcessing) return;
    isStarting = true; // bloquear re-entradas mientras getUserMedia resuelve
    if (!isConfigured) { isStarting = false; ShowSettingsWindow(); return; }

    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    } catch (err) {
      isStarting = false;
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
    isStarting = false;
    setState('recording', 'Grabando');
    await tick(); // esperar a que Svelte monte el canvas
    startWaveform(stream);
  }

  function stopRecording() {
    if (!isRecording || !mediaRecorder) return;
    stopWaveform();
    mediaRecorder.stop();
    isRecording = false;
    if (stream) { stream.getTracks().forEach(t => t.stop()); stream = null; }
  }

  // Cancela la grabación sin transcribir (Escape)
  function cancelRecording() {
    if (!isRecording || !mediaRecorder) return;
    stopWaveform();
    mediaRecorder.onstop = null; // desconecta el handler para no transcribir
    mediaRecorder.stop();
    isRecording = false;
    audioChunks = [];
    if (stream) { stream.getTracks().forEach(t => t.stop()); stream = null; }
    setState(null, 'Listo');
  }

  async function handleRecordingStop() {
    if (isProcessing) return; // guard contra doble ejecución
    isProcessing = true;
    setState('transcribing', 'Transcribiendo');
    const blob = new Blob(audioChunks, { type: 'audio/webm' });
    audioChunks = []; // liberar memoria inmediatamente
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
    } finally {
      isProcessing = false;
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

  function onKeyDown(e) {
    if (e.key === 'Escape' && isRecording) cancelRecording();
  }

  onMount(() => {
    window.addEventListener('keydown', onKeyDown);

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
    Events.On('settings:saved', () => {
      GetSettings().then(s => {
        isConfigured = !!(s.api_key && s.model);
        if (isConfigured) { setState(null, 'Listo'); } else { setUnconfigured(); }
        if (s.hotkey && s.hotkey.display) hotkeyDisplay = s.hotkey.display;
      }).catch(() => {});
    });

    return () => window.removeEventListener('keydown', onKeyDown);
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
      <canvas bind:this={waveformCanvas} class="waveform-canvas" width="38" height="28" aria-hidden="true"></canvas>
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
