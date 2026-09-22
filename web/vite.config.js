import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Override when the API runs elsewhere, e.g. API_PROXY_TARGET=http://localhost:8090
const apiTarget = process.env.API_PROXY_TARGET || 'http://localhost:8080'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
      },
      '/healthz': {
        target: apiTarget,
        changeOrigin: true,
      }
    }
  },
  build: {
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        // Assign by resolved module path. The previous object form only matched
        // the package entry points, so react-dom was hoisted into the recharts
        // chunk and the whole chart library blocked first paint.
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined;
          if (/[\\/]node_modules[\\/](react|react-dom|scheduler)[\\/]/.test(id)) return 'vendor-react';
          if (/[\\/]node_modules[\\/]lucide-react[\\/]/.test(id)) return 'vendor-icons';
          if (/[\\/]node_modules[\\/](recharts|d3-[^\\/]+|victory-vendor|lodash|react-smooth|recharts-scale|decimal\.js-light|eventemitter3|fast-equals|tiny-invariant|internmap|react-is|prop-types)[\\/]/.test(id)) {
            return 'vendor-recharts';
          }
          return undefined;
        }
      }
    }
  }
})
