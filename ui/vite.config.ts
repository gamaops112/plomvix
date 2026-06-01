/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: '/app/',
  server: {
    host: '0.0.0.0',
    port: 3000,
    strictPort: true,
    proxy: {
      '/api':    'http://localhost:8080',
      '/auth':   'http://localhost:8080',
      '/admin':  'http://localhost:8080',
      '/query':  'http://localhost:8080',
      '/ingest': 'http://localhost:8080',
      '/health': 'http://localhost:8080'
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  }
});
