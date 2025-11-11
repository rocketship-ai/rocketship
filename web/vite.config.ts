import path from "path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    host: true,  // Listen on 0.0.0.0 (all interfaces)
    port: 5173,
    strictPort: true,
    origin: 'http://auth.minikube.local',
    allowedHosts: ['auth.minikube.local'],
    hmr: {
      host: 'auth.minikube.local',
      clientPort: 80,
      protocol: 'ws'
    }
  }
})
