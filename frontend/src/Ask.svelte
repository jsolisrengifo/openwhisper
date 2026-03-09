<script>
  import { onMount } from 'svelte';
  import { HideAskWindow } from './bindings/openwhisper/app.js';
  import { Events } from '@wailsio/runtime';

  let responseText = $state('');
  let isVisible = $state(false);

  onMount(() => {
    const cancel = Events.On('ask:response', (text) => {
      responseText = typeof text === 'string' ? text : (text?.data ?? '');
      isVisible = true;
    });

    function onKeyDown(e) {
      if (e.key === 'Escape') close();
    }
    window.addEventListener('keydown', onKeyDown);

    return () => {
      cancel();
      window.removeEventListener('keydown', onKeyDown);
    };
  });

  function close() {
    isVisible = false;
    responseText = '';
    HideAskWindow();
  }
</script>

<div class="ask-shell" style="--wails-draggable:drag">
  <div class="ask-header" style="--wails-draggable:drag">
    <div class="ask-title">
      <svg class="ai-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 14H9V8h2v8zm4 0h-2V8h2v8z"/>
        <path d="M13 2.05v2.02c3.95.49 7 3.85 7 7.93s-3.05 7.44-7 7.93v2.02c5.05-.5 9-4.76 9-9.95s-3.95-9.45-9-9.95z"/>
        <path d="M11 2.05C5.95 2.55 2 6.81 2 12s3.95 9.45 9 9.95v-2.02C7.05 19.44 4 16.08 4 12s3.05-7.44 7-7.93V2.05z"/>
      </svg>
      <span>Respuesta IA</span>
    </div>
    <button class="btn-close" onclick={close} title="Cerrar (Esc)" style="--wails-draggable:no-drag">
      <svg viewBox="0 0 24 24" fill="currentColor">
        <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
      </svg>
    </button>
  </div>

  <div class="ask-body" style="--wails-draggable:no-drag">
    {#if responseText}
      <p class="response-text">{responseText}</p>
    {:else}
      <p class="placeholder">Esperando respuesta…</p>
    {/if}
  </div>

  <div class="ask-footer" style="--wails-draggable:no-drag">
    <button class="btn-dismiss" onclick={close}>Cerrar</button>
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

  .ai-icon {
    width: 16px;
    height: 16px;
    color: #42a5f5;
    flex-shrink: 0;
  }

  .btn-close {
    background: none;
    border: none;
    color: rgba(255,255,255,0.35);
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    border-radius: 6px;
    transition: color 0.12s, background 0.12s;
  }
  .btn-close svg { width: 16px; height: 16px; }
  .btn-close:hover { color: rgba(255,255,255,0.80); background: rgba(255,255,255,0.08); }

  .ask-body {
    flex: 1;
    overflow-y: auto;
    padding: 18px 20px;
    scrollbar-width: thin;
    scrollbar-color: rgba(255,255,255,0.15) transparent;
  }

  .response-text {
    font-size: 13.5px;
    line-height: 1.65;
    color: rgba(255,255,255,0.88);
    white-space: pre-wrap;
    word-break: break-word;
    user-select: text;
    -webkit-user-select: text;
  }

  .placeholder {
    font-size: 13px;
    color: rgba(255,255,255,0.30);
    font-style: italic;
  }

  .ask-footer {
    padding: 10px 16px 14px;
    display: flex;
    justify-content: flex-end;
    border-top: 1px solid rgba(255,255,255,0.07);
    flex-shrink: 0;
  }

  .btn-dismiss {
    background: rgba(255,255,255,0.08);
    border: 1px solid rgba(255,255,255,0.12);
    border-radius: 7px;
    color: rgba(255,255,255,0.65);
    font-size: 12px;
    font-family: inherit;
    padding: 6px 16px;
    cursor: pointer;
    transition: background 0.12s, color 0.12s;
  }
  .btn-dismiss:hover { background: rgba(255,255,255,0.14); color: rgba(255,255,255,0.90); }
</style>
