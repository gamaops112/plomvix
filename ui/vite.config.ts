/// <reference types="vitest" />
import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/app/',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
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
})
