import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'
import vuetify from 'vite-plugin-vuetify'

export default defineConfig({
  base: '/settings-static/',
  build: {
    emptyOutDir: true,
    outDir: process.env.VITE_OUT_DIR || '../internal/app/settings/dist',
  },
  plugins: [vue(), vuetify({ autoImport: true })],
})
