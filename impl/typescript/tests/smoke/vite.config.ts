import { defineConfig } from 'vite'
import { resolve } from 'node:path'

// T001 smoke test vite config.
// Runs on :5174 to stay out of the way of the real demo (:5173).
// Serves smoke.html as the entry and static files from public/.
export default defineConfig({
  root: __dirname,
  publicDir: resolve(__dirname, 'public'),
  server: {
    port: 5174,
    host: '127.0.0.1',
    open: '/smoke.html',
  },
  build: {
    rollupOptions: {
      input: resolve(__dirname, 'smoke.html'),
    },
    outDir: resolve(__dirname, 'dist'),
    emptyOutDir: true,
  },
})
