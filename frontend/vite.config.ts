import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Pass the path through rather than collapsing everything onto "/", so
      // the backend's own routes are reachable. Same-origin in dev also means
      // no CORS handling on the Go side.
      '/api': {
        target: 'http://localhost:8000',
        rewrite: (path) => path.replace(/^\/api/, '') || '/',
      },
    },
  },
})
