import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: path.resolve('index.html'),
        settings: path.resolve('settings.html'),
        ask: path.resolve('ask.html'),
        tts: path.resolve('tts.html'),
      },
    },
  },
});
