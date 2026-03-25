import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    // Inline assets smaller than 4kb instead of emitting separate files
    assetsInlineLimit: 4096,
    rollupOptions: {
      output: {
        // Split vendor code (react, react-dom) into a separate chunk.
        // Wails embeds all chunks anyway, but keeping them separate improves
        // Vite's tree-shaking and makes future bundle analysis easier.
        manualChunks: {
          vendor: ['react', 'react-dom'],
        },
      },
    },
  },
})
