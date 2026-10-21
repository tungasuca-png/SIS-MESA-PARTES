import process from 'node:process'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Acceso desde la red local: el servidor de Vite escucha en todas las
// interfaces y reenvía /api al Gateway, que sigue escuchando solo en
// 127.0.0.1 (igual que los microservicios y PostgreSQL). Así, desde otro
// equipo de la misma red basta abrir http://<IP-del-equipo>:5173 y el
// navegador habla con UN solo origen (sin CORS ni puertos de backend
// expuestos).
//
// Variables (frontend/.env, opcionales):
//   VITE_HOST            interfaz de escucha (por defecto 0.0.0.0; "localhost" = solo este equipo)
//   VITE_PORT            puerto (por defecto 5173)
//   VITE_GATEWAY_TARGET  Gateway al que se reenvía /api (por defecto http://127.0.0.1:8888)
//   VITE_ALLOWED_HOSTS   nombres de equipo permitidos además de las IP, separados por coma
//   VITE_API_URL         vacío = rutas relativas (recomendado; pasa por el proxy)
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const host = env.VITE_HOST || '0.0.0.0'
  const port = Number(env.VITE_PORT) || 5173
  const proxy = {
    '/api': {
      target: env.VITE_GATEWAY_TARGET || 'http://127.0.0.1:8888',
      changeOrigin: true,
    },
  }
  const allowedHosts = (env.VITE_ALLOWED_HOSTS || '')
    .split(',')
    .map((h) => h.trim())
    .filter(Boolean)
  const server = { host, port, proxy, ...(allowedHosts.length ? { allowedHosts } : {}) }

  return {
    plugins: [
      react(),
      tailwindcss(),
    ],
    server,
    // `npm run preview` (build de producción) con la misma configuración de red.
    preview: server,
    // Pruebas de frontend (FASE E-06): vitest + Testing Library sobre jsdom.
    // Los servicios HTTP se simulan en cada prueba: nunca tocan el backend.
    test: {
      environment: 'jsdom',
      include: ['src/**/*.test.{js,jsx}'],
      css: false,
    },
  }
})
