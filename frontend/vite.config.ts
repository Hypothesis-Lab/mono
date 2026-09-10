/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    strictPort: true,
  },
  preview: {
    host: true,
    port: 5173,
    strictPort: true,
    allowedHosts: ['.tailbb2009.ts.net'],
    proxy: {
      '/api': 'http://backend:8000',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    css: false,
    env: {
      VITE_API_BASE_URL: 'http://localhost',
    },
    environmentOptions: {
      jsdom: {
        url: 'http://localhost/',
      },
    },
  },
})
