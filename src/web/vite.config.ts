import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The Go server that `mikado serve` runs; the dev server proxies /api to it.
const apiTarget = process.env.MIKADO_API ?? 'http://127.0.0.1:47291'
// The same hosts `mikado serve` accepts; Vite writes "*.ts.net" as ".ts.net".
const allowedHosts = (process.env.MIKADO_ALLOWED_HOSTS ?? '')
  .split(/[\s,]+/)
  .filter(Boolean)
  .map((h) => h.replace(/^\*\./, '.'))

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  server: {
    proxy: { '/api': apiTarget },
    allowedHosts,
  },
  build: {
    // Built into the Go package that embeds it (go:embed cannot reach outside its package dir).
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    // elkjs (lazy-loaded in its own chunk) is ~1.4 MB on its own; it is served locally.
    chunkSizeWarningLimit: 1600,
  },
})
