import type { CapacitorConfig } from '@capacitor/cli'

// Configurazione dell'app Android (Capacitor). La UI è la stessa build Vite
// dell'app desktop (dist/), servita dalla WebView; il core Go arriva come
// libreria .aar generata da gomobile (vedi mobile/ e android/app/libs).
const config: CapacitorConfig = {
    appId: 'com.renamemusic.app',
    appName: 'RenameMusic',
    webDir: 'dist',
    android: {
        // Nessun contenuto misto: la WebView serve solo gli asset locali.
        allowMixedContent: false,
    },
    plugins: {
        // Dietro status bar e barra di navigazione c'è sempre un fondo scuro:
        // l'header e la striscia inferiore di mobile.css se la WebView è
        // edge-to-edge, altrimenti lo sfondo della finestra (res/values/colors.xml).
        // Icone di sistema quindi chiare ("DARK" = contenuto chiaro su fondo scuro).
        SystemBars: {
            style: 'DARK',
        },
    },
}

export default config
