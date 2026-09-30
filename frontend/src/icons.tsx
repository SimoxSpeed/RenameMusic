import type { ReactNode } from 'react'

// Icone SVG dell'interfaccia (stile "outline", colore dal currentColor del
// contesto). Sono SVG invece di glifi Unicode perché su Windows questi vengono
// resi in modo inconsistente a seconda del font di sistema.

// Icon: attributi comuni a tutte le icone; size e strokeWidth variano da
// un'icona all'altra.
function Icon({ size = 16, strokeWidth = 2, children }: {
    size?: number
    strokeWidth?: number
    children: ReactNode
}) {
    return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            {children}
        </svg>
    )
}

// DownloadIcon: etichetta "Scarica playlist".
export function DownloadIcon() {
    return (
        <Icon>
            <path d="M12 3v12" />
            <path d="M7 10l5 5 5-5" />
            <path d="M4 20h16" />
        </Icon>
    )
}

// PlusIcon: "+" del tasto che porta alle Impostazioni per aggiungere playlist.
export function PlusIcon() {
    return (
        <Icon size={18} strokeWidth={2.2}>
            <path d="M12 5v14M5 12h14" />
        </Icon>
    )
}

// CaretIcon: chevron verso il basso per il trigger del dropdown playlist.
export function CaretIcon() {
    return (
        <Icon size={14}>
            <path d="M6 9l6 6 6-6" />
        </Icon>
    )
}

// ChevronIcon: chevron verso destra (voci navigabili del selettore cartelle).
export function ChevronIcon() {
    return (
        <Icon>
            <path d="M9 6l6 6-6 6" />
        </Icon>
    )
}

// BackIcon: freccia "Indietro" per uscire dalla schermata Impostazioni.
export function BackIcon() {
    return (
        <Icon>
            <path d="M19 12H5" />
            <path d="M12 19l-7-7 7-7" />
        </Icon>
    )
}

// UpIcon: freccia verso l'alto ("cartella superiore" nel selettore cartelle).
export function UpIcon() {
    return (
        <Icon size={18}>
            <path d="M12 19V5" />
            <path d="M5 12l7-7 7 7" />
        </Icon>
    )
}

// RefreshIcon: bottone di ricarica.
export function RefreshIcon() {
    return (
        <Icon>
            <path d="M21 12a9 9 0 1 1-3.5-7.1" />
            <path d="M21 4v6h-6" />
        </Icon>
    )
}

// FolderIcon: cartella chiusa (voci del selettore cartelle).
export function FolderIcon() {
    return (
        <Icon size={18}>
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z" />
        </Icon>
    )
}

// FolderOpenIcon: icona per il pulsante "Apri" (apre la cartella in Esplora risorse).
export function FolderOpenIcon() {
    return (
        <Icon>
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h5a2 2 0 0 1 2 2v1" />
            <path d="M3 9h16.5a1.5 1.5 0 0 1 1.45 1.9l-1.7 6A2 2 0 0 1 17.3 18H4a1.5 1.5 0 0 1-1.5-1.5V9Z" />
        </Icon>
    )
}

// FolderInIcon / FolderOutIcon: cartella con freccia in entrata (partenza) e
// in uscita (destinazione), per distinguere le due righe del riepilogo.
export function FolderInIcon() {
    return (
        <Icon size={18}>
            <path d="M2 9V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H20a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-1" />
            <path d="M2 13h10" />
            <path d="m9 16 3-3-3-3" />
        </Icon>
    )
}

export function FolderOutIcon() {
    return (
        <Icon size={18}>
            <path d="M2 7.5V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H20a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-1.5" />
            <path d="M2 13h10" />
            <path d="m5 10-3 3 3 3" />
        </Icon>
    )
}

// FolderInOutIcon: la cartella delle altre due icone con entrambe le frecce
// sul lato aperto a sinistra, nei loro colori: in entrata (blu, come la
// partenza) sopra e in uscita (verde, come la destinazione) sotto; la
// cartella è blu come quella di partenza. Serve alla
// riga unica quando la destinazione coincide con la partenza.
export function FolderInOutIcon() {
    return (
        <Icon size={18}>
            <path
                style={{ stroke: 'var(--primary)' }}
                d="M2 6.5V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H20a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2"
            />
            <g style={{ stroke: 'var(--primary)' }}>
                <path d="M2 10h8" />
                <path d="m7.5 7.5 2.5 2.5-2.5 2.5" />
            </g>
            <g style={{ stroke: 'var(--accent)' }}>
                <path d="M11 15.5H3" />
                <path d="m5.5 13-2.5 2.5 2.5 2.5" />
            </g>
        </Icon>
    )
}

// InfoCircleIcon: la "i" del pulsante InfoIcon e della scheda "Info" delle
// Impostazioni (più grande).
export function InfoCircleIcon({ size = 14 }: { size?: number }) {
    return (
        <Icon size={size}>
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="11" x2="12" y2="16" />
            <circle cx="12" cy="8" r="0.6" fill="currentColor" />
        </Icon>
    )
}

// KeyboardIcon: glifo per la legenda delle scorciatoie nell'header.
export function KeyboardIcon() {
    return (
        <Icon>
            <rect x="2" y="6" width="20" height="12" rx="2" />
            <path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10" />
        </Icon>
    )
}

// EyeIcon: etichetta "Anteprima".
export function EyeIcon() {
    return (
        <Icon>
            <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7Z" />
            <circle cx="12" cy="12" r="3" />
        </Icon>
    )
}

// ActivityIcon: etichetta "Attività".
export function ActivityIcon() {
    return (
        <Icon>
            <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
        </Icon>
    )
}

// ConvertIcon: etichetta "Risultato conversione" (frecce di scambio).
export function ConvertIcon() {
    return (
        <Icon>
            <path d="M17 3l4 4-4 4" />
            <path d="M21 7H7" />
            <path d="M7 21l-4-4 4-4" />
            <path d="M3 17h14" />
        </Icon>
    )
}

// TrashIcon: azione "Pulisci".
export function TrashIcon() {
    return (
        <Icon>
            <path d="M3 6h18" />
            <path d="M8 6V4h8v2" />
            <path d="M19 6l-1 14H6L5 6" />
        </Icon>
    )
}

// RemoveIcon: azione "Rimuovi" (meno dentro un cerchio); più esplicito del
// cestino per la disinstallazione di yt-dlp.
export function RemoveIcon() {
    return (
        <Icon>
            <circle cx="12" cy="12" r="9" />
            <path d="M8 12h8" />
        </Icon>
    )
}

// TagOffIcon: azione "Cancella tag" (cartellino barrato).
export function TagOffIcon() {
    return (
        <Icon>
            <path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0l-7.2-7.2A2 2 0 0 1 3 12V5a2 2 0 0 1 2-2h7a2 2 0 0 1 1.4.6l7.2 7.2a2 2 0 0 1 0 2.6Z" />
            <circle cx="7.5" cy="7.5" r="1" fill="currentColor" />
            <path d="M4 3 20 21" />
        </Icon>
    )
}

// SettingsIcon: azione "Impostazioni".
export function SettingsIcon() {
    return (
        <Icon>
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z" />
        </Icon>
    )
}

// RulesIcon: scheda "Regole" delle Impostazioni (cursori di regolazione).
export function RulesIcon() {
    return (
        <Icon>
            <path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12" />
            <circle cx="16" cy="6" r="2" />
            <circle cx="10" cy="12" r="2" />
            <circle cx="18" cy="18" r="2" />
        </Icon>
    )
}

// CheckIcon / AlertIcon / CloseIcon: glifi per i toast (successo, errore, chiudi).
export function CheckIcon() {
    return (
        <Icon strokeWidth={2.4}>
            <path d="M20 6 9 17l-5-5" />
        </Icon>
    )
}

export function AlertIcon() {
    return (
        <Icon strokeWidth={2.2}>
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="7" x2="12" y2="13" />
            <circle cx="12" cy="17" r="0.6" fill="currentColor" />
        </Icon>
    )
}

export function CloseIcon() {
    return (
        <Icon size={14}>
            <path d="M18 6 6 18M6 6l12 12" />
        </Icon>
    )
}

// LinkIcon: campo del link da scaricare.
export function LinkIcon() {
    return (
        <Icon>
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
        </Icon>
    )
}
