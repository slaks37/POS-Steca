import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Frontend berkomunikasi dengan backend Go lewat /api. Saat pengembangan,
// permintaan diteruskan ke http://localhost:8080 supaya tidak perlu CORS.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_PROXY ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
