import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 后端端口需与 .env 里的 PORT 保持一致（本机 8080 被占用，改为 8081）
// 需要时可用环境变量覆盖：VITE_API_PROXY=http://localhost:8080 npm run dev
const apiTarget = process.env.VITE_API_PROXY || 'http://localhost:8081'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': apiTarget,
    },
  },
})
