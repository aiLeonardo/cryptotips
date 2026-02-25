import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // 开发时将 /api 请求代理到 Go 后端，避免跨域问题
      '/api': {
        target: 'http://localhost:1024',
        changeOrigin: true,
      },
    },
  },
})
