import React from 'react'
import { createRoot } from 'react-dom/client'
// Font Sora incorporato (file locali serviti da Vite): niente dipendenza dalla
// rete, funziona anche offline. Va importato prima di style.css.
import '@fontsource/sora/400.css'
import '@fontsource/sora/500.css'
import '@fontsource/sora/600.css'
import '@fontsource/sora/700.css'
import './style.css'
import App from './App'

const container = document.getElementById('root')
const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
