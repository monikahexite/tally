import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// In development the Go API runs on :8080; proxying keeps everything
// same-origin, exactly like production where Go serves the built app.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
