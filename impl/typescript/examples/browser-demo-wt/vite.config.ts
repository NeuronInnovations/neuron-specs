import { defineConfig } from 'vite'
import { resolve } from 'node:path'

// 2a-wt — vite config for the WebTransport demo page.
//
// Port 5174 (distinct from Tier 1's 5173 pinned via strictPort) so the
// two demos can run simultaneously on the same laptop.
//
// Plain HTTP on loopback is fine: WebTransport requires HTTPS on the
// *server URL* (handled via certhash on the libp2p dial), not on the
// page origin. Browsers treat http://127.0.0.1:5174 as a secure context
// for mixed-content purposes.
export default defineConfig({
  root: __dirname,
  publicDir: resolve(__dirname, 'public'),
  resolve: {
    alias: [
      { find: 'node:crypto', replacement: resolve(__dirname, 'node-crypto-shim.ts') },
    ],
  },
  server: {
    port: Number(process.env.WT_VITE_PORT ?? 5174),
    host: process.env.VITE_HOST ?? '127.0.0.1',
    strictPort: true,
    open: process.env.VITE_HOST ? false : '/',
    allowedHosts: process.env.VITE_HOST ? true : undefined,
  },
  build: {
    rollupOptions: {
      input: resolve(__dirname, 'index.html'),
    },
    outDir: resolve(__dirname, 'dist'),
    emptyOutDir: true,
  },
})
