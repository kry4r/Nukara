import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 9528,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:19527',
        changeOrigin: true,
      },
    },
  },
})
