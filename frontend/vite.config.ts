import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'https://mental-suzanna-kaizen-labs-09494b30.koyeb.app',
        changeOrigin: true,
      },
      '/webhook': {
        target: 'https://mental-suzanna-kaizen-labs-09494b30.koyeb.app',
        changeOrigin: true,
      },
    },
  },
})
