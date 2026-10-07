// Lint del frontend (npm run lint). Fuori: l'output di build, il progetto
// Android e i binding generati da Wails.
import js from '@eslint/js'
import { defineConfig, globalIgnores } from 'eslint/config'
import reactHooks from 'eslint-plugin-react-hooks'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default defineConfig([
    globalIgnores(['dist', 'android', 'wailsjs']),
    {
        files: ['**/*.{ts,tsx}'],
        extends: [js.configs.recommended, tseslint.configs.recommended],
        plugins: { 'react-hooks': reactHooks },
        // Solo le regole classiche degli hook: le altre del preset servono al
        // React Compiler, che l'app non usa, e vieterebbero schemi voluti (ref
        // aggiornati durante il render per i gestori registrati una volta sola).
        rules: {
            'react-hooks/rules-of-hooks': 'error',
            'react-hooks/exhaustive-deps': 'warn',
        },
        languageOptions: {
            ecmaVersion: 2022,
            globals: globals.browser,
        },
    },
])
