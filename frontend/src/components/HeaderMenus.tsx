import { useEffect, useRef } from 'react'
import { isAndroid, type core } from '../api'
import { ActivityIcon, CaretIcon, CloseIcon, KeyboardIcon, TrashIcon } from '../icons'

// ShortcutsLegend: icona tastiera nell'header con tooltip che elenca le
// scorciatoie disponibili. Riusa lo stile info-icon/info-tooltip (tooltip scuro
// verso il basso) con un contenuto strutturato tasto → azione. In modalità
// semplificata cartelle e anteprima non sono nella schermata principale, quindi
// restano solo le scorciatoie che hanno senso lì.
// separateDest: la destinazione è distinta dalla partenza (altrimenti Ctrl+O
// non fa nulla e non compare). settings: legenda della barra delle
// Impostazioni, con le scorciatoie che valgono lì.
export function ShortcutsLegend({
    simple,
    separateDest,
    settings,
}: {
    simple: boolean
    separateDest: boolean
    settings?: boolean
}) {
    const destShortcut: [string, string][] = separateDest ? [['Ctrl + O', 'Scegli cartella di destinazione']] : []
    const shortcuts: [string, string][] = settings
        ? [
              ['Ctrl + 1…4', 'Vai alla scheda'],
              ['Ctrl + ← / →', 'Scheda precedente / successiva'],
              ['Ctrl + I', 'Scegli cartella di partenza'],
              ...destShortcut,
              ['Ctrl + ,', 'Chiudi le impostazioni'],
              ['Esc', 'Chiudi finestre e impostazioni'],
          ]
        : simple
        ? [
              ['Ctrl + Invio', 'Scarica e converti'],
              ['Ctrl + ,', 'Impostazioni'],
              ['Esc', 'Chiudi finestre e pannelli'],
          ]
        : [
              ['Ctrl + I', 'Scegli cartella di partenza'],
              ...destShortcut,
              ['Ctrl + R', 'Aggiorna scansione'],
              ['Ctrl + Invio', 'Converti / Nuova scansione'],
              ['Ctrl + ,', 'Impostazioni'],
              ['Esc', 'Chiudi finestre e pannelli'],
          ]
    return (
        <button
            type="button"
            className="info-icon shortcuts-legend"
            aria-label="Scorciatoie da tastiera"
            onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
            }}
        >
            <KeyboardIcon />
            <span className="info-tooltip shortcuts-tooltip" role="tooltip">
                <span className="shortcuts-title">Scorciatoie da tastiera</span>
                {shortcuts.map(([keys, desc]) => (
                    <span className="shortcut-row" key={keys}>
                        <kbd>{keys}</kbd>
                        <span className="shortcut-desc">{desc}</span>
                    </span>
                ))}
            </span>
        </button>
    )
}

// DefaultsMenu: solo Android. Un unico tasto "Predefiniti" nella barra delle
// Impostazioni, sulla stessa riga del titolo, che apre un pannello con i due
// tasti veri (Ripristina e Salva, ciascuno con la sua conferma). Lo stato di
// apertura vive in App, così anche il tasto Indietro lo chiude (closeTopmost);
// qui si chiude toccando fuori.
export function DefaultsMenu({
    open,
    onOpenChange,
    onReset,
    onSave,
    disabled,
}: {
    open: boolean
    onOpenChange: (open: boolean) => void
    onReset: () => void
    onSave: () => void
    disabled?: boolean
}) {
    const wrapRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        if (!open) return
        function onDown(e: PointerEvent) {
            if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) onOpenChange(false)
        }
        window.addEventListener('pointerdown', onDown)
        return () => window.removeEventListener('pointerdown', onDown)
    }, [open, onOpenChange])

    return (
        <div className="defaults-menu" ref={wrapRef}>
            <button
                type="button"
                className="header-btn defaults-trigger"
                aria-haspopup="menu"
                aria-expanded={open}
                onClick={() => onOpenChange(!open)}
                disabled={disabled}
            >
                Predefiniti
                <span className={'select-caret' + (open ? ' is-open' : '')}>
                    <CaretIcon />
                </span>
            </button>
            {open && (
                <div className="defaults-popover" role="menu">
                    <p className="defaults-popover-hint">
                        Configurazione di riserva di regole, playlist e modalità semplificata.
                    </p>
                    <button
                        type="button"
                        role="menuitem"
                        onClick={() => {
                            onOpenChange(false)
                            onReset()
                        }}
                        disabled={disabled}
                    >
                        Ripristina predefiniti
                    </button>
                    <button
                        type="button"
                        role="menuitem"
                        className="warn-solid"
                        onClick={() => {
                            onOpenChange(false)
                            onSave()
                        }}
                        disabled={disabled}
                    >
                        Salva predefiniti
                    </button>
                </div>
            )}
        </div>
    )
}

// logKey identifica una riga del registro (ora, tipo e testo) per ricordare
// fin dove è stato letto.
export function logKey(log?: core.LogEntry): string {
    return log ? `${log.time}|${log.kind}|${log.message}` : ''
}

// ActivityMenu: registro Attività come icona nell'header, a sinistra di
// Impostazioni e con lo stesso stile; apre, sopra un velo che oscura tutta la
// pagina, un pannello grande (quasi a tutto schermo su Android) con le righe
// del registro, "Pulisci" e la chiusura. Il pallino segnala righe arrivate
// dopo l'ultima apertura (rosso se tra queste c'è un errore). Come
// DefaultsMenu lo stato di apertura vive in App (Esc e Indietro lo chiudono);
// qui si chiude con la ✕ o toccando il velo. Resta usabile anche durante
// un'operazione: solo "Pulisci" si disabilita.
export function ActivityMenu({
    open,
    onOpenChange,
    logs,
    unseen,
    onClear,
    clearDisabled,
}: {
    open: boolean
    onOpenChange: (open: boolean) => void
    logs: core.LogEntry[]
    unseen: 'none' | 'info' | 'error'
    onClear: () => void
    clearDisabled?: boolean
}) {
    const label = unseen === 'none' ? 'Attività' : 'Attività (nuove righe)'
    return (
        <div className="activity-menu">
            <button
                type="button"
                className={'header-btn activity-trigger' + (open ? ' is-open' : '')}
                aria-haspopup="dialog"
                aria-expanded={open}
                aria-label={label}
                title={isAndroid ? undefined : 'Attività'}
                onClick={() => onOpenChange(!open)}
            >
                <ActivityIcon />
                {unseen !== 'none' && (
                    <span className={'update-dot' + (unseen === 'error' ? ' is-error' : '')} aria-hidden="true" />
                )}
            </button>
            {open && (
                <div className="activity-overlay" onClick={() => onOpenChange(false)}>
                <section
                    className="panel activity-popover"
                    role="dialog"
                    aria-modal="true"
                    aria-label="Attività"
                    onClick={(e) => e.stopPropagation()}
                >
                    <div className="panel-head">
                        <h2>
                            <span className="h2-icon"><ActivityIcon /></span>
                            Attività
                        </h2>
                        <div className="activity-head-actions">
                            <button
                                className="ghost small with-icon"
                                onClick={onClear}
                                disabled={clearDisabled || logs.length === 0}
                            >
                                <span className="btn-icon"><TrashIcon /></span>
                                Pulisci
                            </button>
                            <button
                                type="button"
                                className="ghost small activity-close"
                                onClick={() => onOpenChange(false)}
                                aria-label="Chiudi Attività"
                            >
                                <CloseIcon />
                            </button>
                        </div>
                    </div>
                    <ul className="log">
                        {logs.length === 0 ? (
                            <li className="log-empty">Nessuna attività.</li>
                        ) : (
                            logs.map((log, i) => (
                                <li key={i} className={'log-item log-' + (log.kind || 'info')}>
                                    <span className="log-dot" aria-hidden="true" />
                                    {log.time && <span className="log-time">{log.time}</span>}
                                    <span className="log-msg">{log.message}</span>
                                </li>
                            ))
                        )}
                    </ul>
                </section>
                </div>
            )}
        </div>
    )
}
