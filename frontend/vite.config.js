import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  // Pruebas de frontend (FASE E-06): vitest + Testing Library sobre jsdom.
  // Los servicios HTTP se simulan en cada prueba: nunca tocan el backend.
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{js,jsx}'],
    css: false,
  },
})