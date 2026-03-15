<script>
  import { onMount, tick } from 'svelte';
  import { TranscribeAudio, AskAI, ShowAnswer, PasteText, ShowSettingsWindow, GetSettings, EnableCancelHotkey, DisableCancelHotkey, AddHistoryItem } from '../bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';
  // Non-reactive internal state
  let mediaRecorder = null;
  let audioChunks = [];
  let isRecording = false;
  let isAskMode = false; // true when recording for IA question
  let isStarting = false;  // guard síncrono para la race condition en startRecording async
  let isProcessing = false; // guard: evita doble transcripción/pegado
  let stream = null;

  // Reactive UI state
  let isConfigured = $state(false);
  let uiState = $state(null);

  // Pause state for recording
  let isPaused = $state(false);

  function setState(state, _message) {
    uiState = state;
    // Emit state to Go so the tray icon can be updated.
    Events.Emit('widget:state-change', state ?? 'idle');
  }

  function setUnconfigured() {
    // No visual widget — just mark as unconfigured internally.
  }

  async function startRecording(askMode = false) {
    if (isRecording || isStarting || isProcessing) return;
    isPaused = false;
    isStarting = true;
    if (!isConfigured) { isStarting = false; ShowSettingsWindow(); return; }

    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    } catch (err) {
      isStarting = false;
      setState('error');
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
    isAskMode = askMode;
    isStarting = false;
    if (askMode) Events.Emit('ask:new-chat');
    EnableCancelHotkey();
    setState('recording');
  }

  function stopRecording() {
    if (!isRecording || !mediaRecorder) return;
    isPaused = false;
    DisableCancelHotkey();
    mediaRecorder.stop();
    isRecording = false;
    if (stream) { stream.getTracks().forEach(t => t.stop()); stream = null; }
  }

  function pauseRecording() {
    if (!isRecording || !mediaRecorder || isPaused) return;
    mediaRecorder.pause();
    isPaused = true;
    setState('paused');
  }

  function resumeRecording() {
    if (!isRecording || !mediaRecorder || !isPaused) return;
    mediaRecorder.resume();
    isPaused = false;
    setState('recording');
  }

  function togglePause() {
    if (isPaused) resumeRecording(); else pauseRecording();
  }

  // Cancela la grabación sin transcribir (Escape global desde Go)
  function cancelRecording() {
    if (!isRecording || !mediaRecorder) return;
    isPaused = false;
    DisableCancelHotkey();
    mediaRecorder.onstop = null;
    mediaRecorder.stop();
    isRecording = false;
    audioChunks = [];
    if (stream) { stream.getTracks().forEach(t => t.stop()); stream = null; }
    setState(null);
  }

  async function handleRecordingStop() {
    if (isProcessing) return;
    isProcessing = true;
    const wasAskMode = isAskMode;
    isAskMode = false;

    setState('transcribing');

    const blob = new Blob(audioChunks, { type: 'audio/webm' });
    audioChunks = [];
    const mimeType = blob.type || 'audio/webm';

    try {
      const base64 = await blobToBase64(blob);

      if (wasAskMode) {
        const answer = await AskAI(base64, mimeType);
        if (!answer || answer.trim() === '') {
          setState('error');
          setTimeout(() => setState(null), 3000);
          return;
        }
        await ShowAnswer(answer.trim());
        AddHistoryItem(answer.trim(), 'ai').catch(() => {});
        setState('done');
        setTimeout(() => setState(null), 2000);
      } else {
        const text = await TranscribeAudio(base64, mimeType);
        if (!text || text.trim() === '') {
          setState('error');
          setTimeout(() => setState(null), 3000);
          return;
        }
        await PasteText(text.trim());
        AddHistoryItem(text.trim(), 'transcription').catch(() => {});
        setState('done');
        setTimeout(() => setState(null), 2000);
      }
    } catch (err) {
      setState('error');
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
    if (isRecording) { stopRecording(); } else { startRecording(false); }
  }

  function toggleAskRecording() {
    if (isRecording && isAskMode) { stopRecording(); }
    else if (!isRecording) { startRecording(true); }
  }

  onMount(() => {
    GetSettings().then(s => {
      isConfigured = !!(s.api_key && s.model);
      if (!isConfigured) setUnconfigured();
    }).catch(() => {
      isConfigured = false;
      setUnconfigured();
    });

    const cancelToggleRecording    = Events.On('toggle-recording', toggleRecording);
    const cancelToggleAskRecording  = Events.On('toggle-ask-recording', toggleAskRecording);
    const cancelCancelRecording     = Events.On('cancel-recording', cancelRecording);
    const cancelOpenSettings        = Events.On('open-settings', () => ShowSettingsWindow());
    const cancelTTSProcessing       = Events.On('tts:processing', () => {
      setState('transcribing');
    });
    const cancelTTSAudio            = Events.On('tts:audio', () => {
      setState('done');
      setTimeout(() => setState(null), 2500);
    });
    const cancelTTSError            = Events.On('tts:error', () => {
      setState('error');
      setTimeout(() => setState(null), 4000);
    });
    const cancelSettingsSaved       = Events.On('settings:saved', () => {
      GetSettings().then(s => {
        isConfigured = !!(s.api_key && s.model);
        if (isConfigured) { setState(null); } else { setUnconfigured(); }
      }).catch(() => {});
    });

    return () => {
      cancelToggleRecording();
      cancelToggleAskRecording();
      cancelCancelRecording();
      cancelOpenSettings();
      cancelTTSProcessing();
      cancelTTSAudio();
      cancelTTSError();
      cancelSettingsSaved();
    };
  });
</script>

<!-- No visible UI — the system tray icon is the interface now. -->
<div style="display:none"></div>
