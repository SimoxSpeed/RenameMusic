import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
    base: './',
    plugins: [react()],
    build: {
        // CSS minificato con esbuild come prima di Vite 8: Lightning CSS (il
        // nuovo predefinito) riscrive colori, unicode-range e color-scheme, e
        // il passaggio va verificato a parte su desktop e Android.
        cssMinify: 'esbuild',
    },
})
