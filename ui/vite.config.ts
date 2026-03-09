import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
    }),
    Components({
      resolvers: [ElementPlusResolver()],
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    proxy: {
      '/api': {
        //target: 'http://localhost:8001',
        target: 'http://172.29.64.1:8001',
        changeOrigin: true
      },
      '/engine': {
        //target: 'http://localhost:8080',
        target: 'http://172.29.64.1:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/engine/, '')
      }
    }
  }
})
