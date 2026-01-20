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
  optimizeDeps: {
    include: [
      "@bufbuild/protobuf",
      "@connectrpc/connect",
      "@connectrpc/connect-web"
    ],
  },
  server: {
    host: true,  // Listen on all interfaces (required for minikube access)
    port: 5173,
    strictPort: true,
    origin: 'http://auth.minikube.local',
    allowedHosts: ['auth.minikube.local', 'host.docker.internal'],
    hmr: {
      host: 'auth.minikube.local',
      clientPort: 80,
      protocol: 'ws'
    }
  }
})
