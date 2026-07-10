import { useEffect, useRef, useState, type ReactNode } from 'react'
import './App.css'
import {
    GetState,
    SelectFolder,
    Scan,
    ProcessAll,
    SetConfig,
    ResetConfig,
    SetAsDefault,
    ClearLogs,
    ChooseDirectory,
    SetOptions,
    SetWatchEnabled,
    OpenFolder,
    ClearTags,
    Cancel,
    SetPlaylists,
    DownloadPlaylist,
    InstallYtDlp,
    UninstallYtDlp,
    SetYtDlpConfig,
    ChooseYtDlpFile,
} from '../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../wailsjs/runtime/runtime'
import { main, rules, playlist } from '../wailsjs/go/models'

type Status = { message: string; ok: boolean }

// Toast: notifica effimera in basso a destra. `ok` decide colore/icona.
type Toast = { id: number; ok: boolean; message: string }

function listToText(list: string[] | undefined): string {
    return (list ?? []).join('\n')
}

function textToList(text: string): string[] {
    return text.split('\n')
}

// DownloadIcon: etichetta "Scarica playlist".
function DownloadIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M12 3v12" />
            <path d="M7 10l5 5 5-5" />
            <path d="M4 20h16" />
        </svg>
    )
}

// CaretIcon: chevron verso il basso per il trigger del dropdown playlist.
function CaretIcon() {
    return (
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M6 9l6 6 6-6" />
        </svg>
    )
}

// BackIcon: freccia "Indietro" per uscire dalla schermata Impostazioni.
function BackIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M19 12H5" />
            <path d="M12 19l-7-7 7-7" />
        </svg>
    )
}

// RefreshIcon: icona SVG per il bottone di ricarica; sostituisce il glifo Unicode
// che su Windows viene reso in modo inconsistente a seconda del font di sistema.
function RefreshIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M21 12a9 9 0 1 1-3.5-7.1" />
            <path d="M21 4v6h-6" />
        </svg>
    )
}

// FolderOpenIcon: icona per il pulsante "Apri" (apre la cartella in Esplora risorse).
function FolderOpenIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h5a2 2 0 0 1 2 2v1" />
            <path d="M3 9h16.5a1.5 1.5 0 0 1 1.45 1.9l-1.7 6A2 2 0 0 1 17.3 18H4a1.5 1.5 0 0 1-1.5-1.5V9Z" />
        </svg>
    )
}

// InfoIcon: pulsante con tooltip custom. \u00c8 un <button> per essere focusabile
// da tastiera e per catturare il click impedendo che tocchi la <label> genitore
// (altrimenti cliccare la "i" attiverebbe la checkbox associata).
function InfoIcon({ text }: { text: string }) {
    return (
        <button
            type="button"
            className="info-icon"
            aria-label={text}
            onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
            }}
        >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="11" x2="12" y2="16" />
                <circle cx="12" cy="8" r="0.6" fill="currentColor" />
            </svg>
            <span className="info-tooltip" role="tooltip">{text}</span>
        </button>
    )
}

// KeyboardIcon: glifo per la legenda delle scorciatoie nell'header.
function KeyboardIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <rect x="2" y="6" width="20" height="12" rx="2" />
            <path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10" />
        </svg>
    )
}

// ShortcutsLegend: icona tastiera nell'header con tooltip che elenca le
// scorciatoie disponibili. Riusa lo stile info-icon/info-tooltip (tooltip scuro
// verso il basso) con un contenuto strutturato tasto → azione.
function ShortcutsLegend() {
    const shortcuts: [string, string][] = [
        ['Ctrl + O', 'Scegli cartella'],
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

// Tooltip avvolge un elemento e mostra un fumetto informativo moderno (stesso
// stile del tooltip "i"/scorciatoie) su hover o focus. Essendo un wrapper, il
// fumetto compare anche quando l'elemento interno è disabilitato (i bottoni
// disabilitati non ricevono hover, ma il wrapper sì): utile per spiegare PERCHÉ
// un'azione non è disponibile. `grow` fa espandere il wrapper nei contenitori
// flex (es. il percorso cartella che deve riempire la toolbar).
function Tooltip({ label, children, grow }: { label: string; children: ReactNode; grow?: boolean }) {
    if (!label) return <>{children}</>
    return (
        <span className={'tip' + (grow ? ' tip-grow' : '')}>
            {children}
            <span className="info-tooltip" role="tooltip">{label}</span>
        </span>
    )
}

// PlaylistSelect: dropdown custom per la scelta della playlist da scaricare.
// Il <select> nativo apre una lista disegnata dal WebView (aspetto "di sistema",
// non stilabile): qui la sostituiamo con una lista nostra (angoli arrotondati,
// ombra, colori del tema) mantenendo il trigger identico alla vecchia select
// chiusa. Chiude su click-fuori ed Esc.
function PlaylistSelect({
    value,
    options,
    onChange,
    disabled,
}: {
    value: string
    options: { name: string }[]
    onChange: (name: string) => void
    disabled?: boolean
}) {
    const [open, setOpen] = useState(false)
    const wrapRef = useRef<HTMLDivElement>(null)

    // I listener (click-fuori ed Esc) vivono solo mentre la lista è aperta.
    useEffect(() => {
        if (!open) return
        function onDown(e: MouseEvent) {
            if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false)
        }
        function onKey(e: KeyboardEvent) {
            if (e.key === 'Escape') setOpen(false)
        }
        window.addEventListener('mousedown', onDown)
        window.addEventListener('keydown', onKey)
        return () => {
            window.removeEventListener('mousedown', onDown)
            window.removeEventListener('keydown', onKey)
        }
    }, [open])

    const empty = options.length === 0
    const label = empty ? 'Nessuna playlist' : value || 'Seleziona playlist'

    return (
        <div className="playlist-select-wrap" ref={wrapRef}>
            <button
                type="button"
                className="playlist-select"
                aria-haspopup="listbox"
                aria-expanded={open}
                disabled={disabled}
                onClick={() => setOpen((o) => !o)}
            >
                <span className="playlist-select-value">{label}</span>
                <span className={'playlist-select-caret' + (open ? ' is-open' : '')}>
                    <CaretIcon />
                </span>
            </button>
            {open && !empty && (
                <ul className="playlist-menu" role="listbox">
                    {options.map((p) => (
                        <li
                            key={p.name}
                            role="option"
                            aria-selected={p.name === value}
                            className={'playlist-option' + (p.name === value ? ' is-selected' : '')}
                            onClick={() => {
                                onChange(p.name)
                                setOpen(false)
                            }}
                        >
                            {p.name}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    )
}

// splitName separa il nome file dalla sua estensione (parte dopo l'ultimo punto).
// Restituisce base senza estensione ed estensione senza punto. Usato per NON
// mostrare mai l'estensione nei nomi file: quella viaggia in un chip a parte.
function splitName(full: string): { base: string; ext: string } {
    const idx = full.lastIndexOf('.')
    if (idx <= 0 || idx === full.length - 1) return { base: full, ext: '' }
    return { base: full.slice(0, idx), ext: full.slice(idx + 1).toLowerCase() }
}

// ExtChip mostra il formato del file in stile blu. I file trattati sono sempre
// mp3 (nessuna conversione tra formati diversi), quindi mostriamo semplicemente
// l'estensione: serve solo perché l'estensione non è mai visibile nei nomi.
function ExtChip({ ext }: { ext: string }) {
    if (!ext) return null
    return <span className="ext-chip ext-same">{ext}</span>
}

// tagChanged confronta il tag ID3 attuale con quello che verrebbe scritto.
function tagChanged(current: string | undefined, expected: string | undefined): boolean {
    return (current ?? '') !== (expected ?? '')
}

// fileWillChange indica se un file subirà una qualsiasi modifica: il nome
// cambia, oppure (per gli MP3) cambia il titolo o l'artista scritto nei tag.
// Usata sia dal filtro "Solo da modificare" sia dai badge di stato.
function fileWillChange(f: main.FileView): boolean {
    const nameChanged = splitName(f.name).base !== splitName(f.preview).base
    const tagsChanged = !!f.mp3 && (tagChanged(f.title, f.titlePreview) || tagChanged(f.artist, f.artistPreview))
    return nameChanged || tagsChanged
}

// CurrentField mostra una riga "etichetta: valore" nella colonna File attuale
// (nome/titolo/artista impilati in una sola cella). Se il valore sta per
// cambiare compare sbarrato e attenuato (stessa classe .old-name della colonna
// Nome); se manca del tutto (tag non presente) mostra un placeholder neutro.
function CurrentField({ label, value, changed }: { label: string; value: string; changed: boolean }) {
    return (
        <div className="current-line">
            <span className="current-label">{label}:</span>{' '}
            {!value ? (
                <span className="muted-dash">nessun tag</span>
            ) : changed ? (
                <s className="old-name">{value}</s>
            ) : (
                value
            )}
        </div>
    )
}

// ErrorLabel: chip rosso "Errore" nella colonna esito. Non è un elemento
// interattivo (niente click/animazione): serve solo a mostrare un tooltip al
// passaggio del mouse o al focus da tastiera, così la tabella resta compatta
// ma il dettaglio è a un hover di distanza.
function ErrorLabel({ message }: { message: string }) {
    return (
        <span className="result-error" tabIndex={0} aria-label={'Errore: ' + message}>
            <AlertIcon />
            Errore
            <span className="result-error-tip" role="tooltip">{message}</span>
        </span>
    )
}

// EyeIcon: etichetta "Anteprima".
function EyeIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7Z" />
            <circle cx="12" cy="12" r="3" />
        </svg>
    )
}

// ActivityIcon: etichetta "Attività".
function ActivityIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
        </svg>
    )
}

// ConvertIcon: etichetta "Risultato conversione" (frecce di scambio).
function ConvertIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M17 3l4 4-4 4" />
            <path d="M21 7H7" />
            <path d="M7 21l-4-4 4-4" />
            <path d="M3 17h14" />
        </svg>
    )
}

// TrashIcon: azione "Pulisci".
function TrashIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M3 6h18" />
            <path d="M8 6V4h8v2" />
            <path d="M19 6l-1 14H6L5 6" />
        </svg>
    )
}

// RemoveIcon: azione "Rimuovi" (meno dentro un cerchio); più esplicito del
// cestino per la disinstallazione di yt-dlp.
function RemoveIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="9" />
            <path d="M8 12h8" />
        </svg>
    )
}

// TagOffIcon: azione "Cancella tag" (cartellino barrato).
function TagOffIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0l-7.2-7.2A2 2 0 0 1 3 12V5a2 2 0 0 1 2-2h7a2 2 0 0 1 1.4.6l7.2 7.2a2 2 0 0 1 0 2.6Z" />
            <circle cx="7.5" cy="7.5" r="1" fill="currentColor" />
            <path d="M4 3 20 21" />
        </svg>
    )
}

// SettingsIcon: azione "Impostazioni".
function SettingsIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z" />
        </svg>
    )
}

// CheckIcon / AlertIcon / CloseIcon: glifi per i toast (successo, errore, chiudi).
function CheckIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M20 6 9 17l-5-5" />
        </svg>
    )
}

function AlertIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="7" x2="12" y2="13" />
            <circle cx="12" cy="17" r="0.6" fill="currentColor" />
        </svg>
    )
}

function CloseIcon() {
    return (
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M18 6 6 18M6 6l12 12" />
        </svg>
    )
}

function cloneConfig(cfg: rules.Config): rules.Config {
    return {
        startFolder: cfg.startFolder,
        supportedExtensions: [...(cfg.supportedExtensions ?? [])],
        occurrenciesToRemove: [...(cfg.occurrenciesToRemove ?? [])],
        occurrenciesToReplaceWithFt: [...(cfg.occurrenciesToReplaceWithFt ?? [])],
        replacements: (cfg.replacements ?? []).map((r) => ({ from: r.from, to: r.to })),
        artistExceptions: [...(cfg.artistExceptions ?? [])],
    } as rules.Config
}

function App() {
    const [state, setState] = useState<main.StateResponse | null>(null)
    const [status, setStatus] = useState<Status>({ message: '', ok: true })
    const [toasts, setToasts] = useState<Toast[]>([])
    const toastIdRef = useRef(0)
    const [busy, setBusy] = useState(false)
    const [showSettings, setShowSettings] = useState(false)
    const [draft, setDraft] = useState<rules.Config | null>(null)
    // playlistDraft: bozza in editing dell'elenco playlist YouTube (Impostazioni),
    // salvata a parte da SetPlaylists (non fa parte di rules.Config).
    const [playlistDraft, setPlaylistDraft] = useState<playlist.Playlist[]>([])
    // selectedPlaylist: nome scelto nel select accanto al bottone "Scarica".
    const [selectedPlaylist, setSelectedPlaylist] = useState('')
    const [results, setResults] = useState<main.ResultView[] | null>(null)
    const [confirmDefault, setConfirmDefault] = useState(false)
    const [destSameAsSource, setDestSameAsSource] = useState(true)
    const [destFolder, setDestFolder] = useState('')
    const [deleteOriginals, setDeleteOriginals] = useState(false)
    const [watchEnabled, setWatchEnabled] = useState(false)
    // Gestione di yt-dlp: ytDlpManaged rispecchia la checkbox "Gestisci
    // autonomamente"; ytDlpPathDraft è il campo del percorso personalizzato,
    // persistito con SetYtDlpConfig (all'uscita dal campo o via "Sfoglia").
    const [ytDlpManaged, setYtDlpManaged] = useState(true)
    const [ytDlpPathDraft, setYtDlpPathDraft] = useState('')
    const [confirmDeleteOriginals, setConfirmDeleteOriginals] = useState(false)
    const [confirmClearTags, setConfirmClearTags] = useState(false)
    // confirmInstallYtDlp: popup chiesto quando si preme "Scarica" playlist ma
    // yt-dlp non è presente. Il testo cambia a seconda di ytDlpManaged: se attivo
    // chiede solo il permesso di scaricarlo, altrimenti propone di attivare la
    // gestione automatica e procedere.
    const [confirmInstallYtDlp, setConfirmInstallYtDlp] = useState(false)
    // confirmUninstallYtDlp: popup di conferma per rimuovere la copia gestita.
    const [confirmUninstallYtDlp, setConfirmUninstallYtDlp] = useState(false)
    // confirmDownloadYtDlp: popup di avvertimento prima di scaricare/installare
    // yt-dlp dal tasto dedicato (quando non è presente), separato dal flusso di
    // download di una playlist (confirmInstallYtDlp).
    const [confirmDownloadYtDlp, setConfirmDownloadYtDlp] = useState(false)
    // confirmLeaveSettings: popup chiesto premendo "Indietro" nelle Impostazioni
    // quando ci sono modifiche non salvate (salva / scarta / annulla).
    const [confirmLeaveSettings, setConfirmLeaveSettings] = useState(false)
    // progress: avanzamento dell'ultima elaborazione (x/totale), popolato dagli
    // eventi process:progress durante ProcessAll; null quando non pertinente.
    const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)
    // showOnlyChanged: vista dell'anteprima limitata ai soli file che cambieranno
    // nome. È SOLO una vista: l'elaborazione tratta comunque tutti i file.
    const [showOnlyChanged, setShowOnlyChanged] = useState(false)
    // cancellable: true mentre è in corso un'operazione interrompibile (ProcessAll
    // o ClearTags), così mostriamo il tasto "Annulla".
    const [cancellable, setCancellable] = useState(false)
    // booted diventa true al termine del caricamento iniziale: finché è false
    // mostriamo un placeholder di caricamento invece del messaggio "vuoto",
    // così al refresh non si vede un lampo di stato vuoto prima dei dati.
    const [booted, setBooted] = useState(false)

    function absorb(resp: main.ActionResponse) {
        setState(resp.state)
        if (resp.state.config) {
            setDraft(cloneConfig(resp.state.config))
        }
        const playlists = resp.state.playlists ?? []
        setPlaylistDraft(playlists.map((p) => ({ name: p.name, url: p.url })))
        setSelectedPlaylist((prev) => (playlists.some((p) => p.name === prev) ? prev : (playlists[0]?.name ?? '')))
    }

    // Durata minima (ms) per cui lo stato "busy" resta attivo una volta partito:
    // così la barra di progresso non lampeggia (accendendosi e spegnendosi in
    // pochi ms) sulle operazioni rapide, ma resta visibile un istante coerente.
    const MIN_BUSY_MS = 450

    async function guard(fn: () => Promise<void>) {
        const start = performance.now()
        setBusy(true)
        setStatus({ message: 'Operazione in corso...', ok: true })
        try {
            await fn()
        } catch (err: any) {
            setStatus({ message: 'Errore: ' + (err?.message ?? String(err)), ok: false })
        } finally {
            const elapsed = performance.now() - start
            if (elapsed < MIN_BUSY_MS) {
                await new Promise((r) => window.setTimeout(r, MIN_BUSY_MS - elapsed))
            }
            setBusy(false)
        }
    }

    // notify mostra un toast effimero. Gli errori restano più a lungo (portano
    // un messaggio da leggere); i successi spariscono in fretta. Messaggio vuoto
    // => nessun toast.
    function notify(ok: boolean, message: string) {
        if (!message) return
        const id = (toastIdRef.current += 1)
        setToasts((prev) => [...prev, { id, ok, message }])
        window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id))
        }, ok ? 3500 : 6000)
    }

    function dismissToast(id: number) {
        setToasts((prev) => prev.filter((t) => t.id !== id))
    }

    function syncOptions(s: main.StateResponse) {
        setDestSameAsSource(s.destinationSameAsSource)
        setDestFolder(s.destinationFolder ?? '')
        setDeleteOriginals(s.deleteOriginals)
        setWatchEnabled(s.watchEnabled)
        setYtDlpManaged(s.ytDlpManaged)
        setYtDlpPathDraft(s.ytDlpPath ?? '')
    }

    // Carica lo stato iniziale (cartella + opzioni + anteprima) in UN SOLO passaggio:
    // il backend (GetState) scansiona già la cartella ricordata e restituisce le
    // anteprime, quindi la UI si popola una sola volta senza svuotarsi/riempirsi.
    // Il ref evita la doppia esecuzione indotta da React.StrictMode in dev.
    const bootedRef = useRef(false)
    useEffect(() => {
        if (bootedRef.current) return
        bootedRef.current = true
        guard(async () => {
            const resp = await GetState()
            absorb(resp)
            syncOptions(resp.state)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        }).finally(() => setBooted(true))
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Aggiorna e persiste le opzioni di elaborazione (destinazione + eliminazione originali).
    function applyOptions(same: boolean, dest: string, del: boolean) {
        setDestSameAsSource(same)
        setDestFolder(dest)
        setDeleteOriginals(del)
        SetOptions(same, dest, del).catch(() => {
            /* la persistenza opzioni non deve bloccare la UI */
        })
    }

    // In modalità watch, il backend rileva variazioni nella cartella e ci
    // manda lo stato aggiornato: aggiorniamo solo l'anteprima (la conversione
    // resta manuale). Se l'utente sta guardando i risultati dell'ultima
    // conversione, ignoriamo l'evento: il backend è già in pausa in quel caso,
    // ma è una difesa extra lato UI.
    useEffect(() => {
        EventsOn('watch:changed', (payload: unknown) => {
            const next = payload as main.StateResponse
            if (!next) return
            setState((prev) => (prev ? ({ ...prev, files: next.files, logs: next.logs } as main.StateResponse) : next))
        })
        return () => {
            EventsOff('watch:changed')
        }
    }, [])

    // Trascinamento di una cartella sulla finestra: il backend imposta la
    // cartella di partenza e ci manda lo stato aggiornato (con l'anteprima).
    // Assorbiamo tutto come farebbe una scansione manuale.
    useEffect(() => {
        EventsOn('folder:dropped', (payload: unknown) => {
            const next = payload as main.StateResponse
            if (!next) return
            setState(next)
            if (next.config) setDraft(cloneConfig(next.config))
            setResults(null)
            syncOptions(next)
            const ok = next.folder !== ''
            const msg = ok ? 'Cartella impostata dal trascinamento.' : 'Trascinamento non valido.'
            setStatus({ message: msg, ok })
            notify(ok, msg)
        })
        return () => {
            EventsOff('folder:dropped')
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Avanzamento di ProcessAll: il backend emette un evento per ogni file
    // completato; aggiorniamo il contatore mostrato accanto a "Operazione in corso".
    useEffect(() => {
        EventsOn('process:progress', (payload: unknown) => {
            const p = payload as { done: number; total: number } | null
            if (p) setProgress(p)
        })
        return () => {
            EventsOff('process:progress')
        }
    }, [])

    function toggleWatch(next: boolean) {
        guard(async () => {
            const resp = await SetWatchEnabled(next)
            absorb(resp)
            setWatchEnabled(resp.state.watchEnabled)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        })
    }

    // Riscansiona la cartella corrente (utile se il contenuto è cambiato).
    function refresh() {
        guard(async () => {
            const resp = await Scan()
            absorb(resp)
            setResults(null)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        })
    }

    // Scegli cartella: imposta il percorso e mostra subito l'anteprima (scan automatico).
    function chooseFolder() {
        guard(async () => {
            const selected = await SelectFolder()
            if (!selected.ok) {
                absorb(selected)
                setResults(null)
                setStatus({ message: selected.message ?? '', ok: selected.ok })
                return
            }
            const scanned = await Scan()
            absorb(scanned)
            setResults(null)
            setStatus({ message: scanned.message ?? '', ok: scanned.ok })
        })
    }

    function chooseDestination() {
        guard(async () => {
            const path = await ChooseDirectory()
            if (path) {
                applyOptions(destSameAsSource, path, deleteOriginals)
                setStatus({ message: 'Cartella di destinazione impostata.', ok: true })
            } else {
                // Selezione annullata: manteniamo la destinazione precedente e
                // sblocchiamo lo stato (altrimenti resterebbe "Operazione in corso...").
                // ok:false → messaggio in rosso, coerente con l'annullamento della
                // cartella di partenza.
                setStatus({ message: 'Selezione annullata.', ok: false })
            }
        })
    }

    // Apre una cartella nel file manager di sistema (Esplora risorse). Non è
    // un'operazione bloccante: non usiamo `guard` per non accendere la barra.
    // In caso di errore mostriamo un toast (utile anche a diagnosticare: se il
    // binding non fosse disponibile, la Promise verrebbe rifiutata).
    function openFolder(path: string) {
        if (!path) return
        Promise.resolve(OpenFolder(path))
            .then((resp) => {
                if (resp && !resp.ok) notify(false, resp.message || 'Impossibile aprire la cartella.')
            })
            .catch((err) => notify(false, 'Impossibile aprire la cartella: ' + (err?.message ?? String(err))))
    }

    // Processo unificato: normalizzazione nomi + scrittura tag in un colpo solo.
    // Usa le opzioni persistite lato backend.
    function process() {
        if (!destSameAsSource && destFolder === '') {
            setStatus({ message: 'Scegli una cartella di destinazione o riattiva "uguale alla partenza".', ok: false })
            return
        }
        setProgress(null)
        setCancellable(true)
        guard(async () => {
            const resp = await ProcessAll()
            // Operazione conclusa: non è più annullabile (evita che un click sul
            // tasto Annulla durante la coda "busy" lasci lo status appeso).
            setCancellable(false)
            absorb(resp)
            setResults(resp.results ?? [])
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        }).finally(() => {
            setProgress(null)
            setCancellable(false)
        })
    }

    // Cancella TUTTI i tag ID3 dagli MP3 della cartella (azione distruttiva:
    // confermata da un popup). Non rinomina nulla, agisce in posto.
    function confirmClearTagsAction() {
        setConfirmClearTags(false)
        setProgress(null)
        setCancellable(true)
        guard(async () => {
            const resp = await ClearTags()
            setCancellable(false)
            absorb(resp)
            setResults(null)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        }).finally(() => {
            setProgress(null)
            setCancellable(false)
        })
    }

    // Richiede al backend di interrompere l'operazione in corso (conversione o
    // cancellazione tag). Il backend si ferma tra un file e l'altro; la Promise
    // dell'operazione si risolve poi con l'esito parziale.
    function cancelOp() {
        // Azzeriamo subito il contatore: altrimenti "(x/totale)" resterebbe
        // congelato finché l'operazione non ritorna, mascherando il messaggio.
        setProgress(null)
        setStatus({ message: 'Annullamento in corso…', ok: false })
        Cancel().catch(() => {
            /* l'annullamento non deve generare errori bloccanti in UI */
        })
    }

    function resetConfig() {
        guard(async () => {
            const resp = await ResetConfig()
            absorb(resp)
            setResults(null)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Pulisce solo le attività stampate: non ripristina né salva altro stato (folder/regole/anteprima).
    async function clearLogs() {
        try {
            const resp = await ClearLogs()
            setState((prev) => (prev ? ({ ...prev, logs: resp.state.logs } as main.StateResponse) : prev))
        } catch {
            /* niente da fare: la pulizia log non deve disturbare lo stato */
        }
    }

    // Riporta regole e playlist ai valori salvati (scarta tutte le modifiche non
    // ancora salvate delle Impostazioni, comprese quelle alle playlist).
    function revertDraft() {
        if (!state?.config) return
        setDraft(cloneConfig(state.config))
        setPlaylistDraft((state?.playlists ?? []).map((p) => ({ name: p.name, url: p.url })))
        setStatus({ message: 'Ripristinate le impostazioni salvate.', ok: true })
    }

    // Rende regole e playlist in editing il nuovo predefinito (dopo conferma dal
    // popup). Non tocca i valori correnti né i draft in editing: aggiorna solo il
    // log/stato.
    function confirmMakeDefault() {
        if (!draft) return
        setConfirmDefault(false)
        guard(async () => {
            const resp = await SetAsDefault(draft, playlistDraft)
            setState((prev) => (prev ? ({ ...prev, logs: resp.state.logs } as main.StateResponse) : resp.state))
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        })
    }

    const folder = state?.folder ?? ''
    const files = state?.files ?? []
    const logs = state?.logs ?? []
    const playlists = state?.playlists ?? []
    // settingsDirty: true se ci sono modifiche non salvate nelle Impostazioni,
    // ossia le regole in editing (draft) differiscono da quelle salvate, oppure
    // l'elenco playlist in editing differisce da quello salvato. Il confronto usa
    // cloneConfig su entrambi i lati per normalizzare l'ordine dei campi.
    const draftDirty =
        !!draft && !!state?.config && JSON.stringify(cloneConfig(draft)) !== JSON.stringify(cloneConfig(state.config))
    const playlistsDirty =
        JSON.stringify(playlistDraft) !== JSON.stringify(playlists.map((p) => ({ name: p.name, url: p.url })))
    const settingsDirty = draftDirty || playlistsDirty
    // Contatori nell'header: dopo un'elaborazione la lista `files` \u00e8 vuota
    // (i file sono stati rinominati/spostati), quindi mostreremmo "0 file".
    // Quando ci sono `results` calcoliamo i contatori da quelli, cos\u00ec l'utente
    // vede il riepilogo di ci\u00f2 che \u00e8 appena stato fatto.
    const showingResults = results !== null
    const fileCount = showingResults ? results!.length : files.length
    const mp3Count = showingResults
        ? results!.filter((r) => r.tagged).length
        : files.filter((f) => f.mp3).length
    const toRenameCount = showingResults
        ? results!.filter((r) => !r.skipped && !r.failed && r.oldName !== r.newName).length
        : files.filter((f) => f.preview !== f.name).length
    const failedCount = showingResults ? results!.filter((r) => r.failed).length : 0
    const destReady = destSameAsSource || destFolder !== ''
    const canProcess = !busy && folder !== '' && files.length > 0 && destReady
    // "Cancella tag" agisce in posto sugli MP3 scansionati: serve almeno un MP3.
    const canClearTags = !busy && files.some((f) => f.mp3)
    // Anteprima filtrata: se il toggle è attivo, mostra solo i file che
    // subiranno UNA QUALSIASI modifica — nel nome oppure nei tag ID3
    // (titolo/artista) — non solo quelli da rinominare. Resta comunque solo una
    // vista: l'elaborazione tratta sempre tutti i file.
    const previewFiles = showOnlyChanged ? files.filter(fileWillChange) : files

    // Attiva/disattiva "Elimina originali" con conferma esplicita quando si passa
    // da OFF a ON (è un'azione distruttiva). Spegnerlo non richiede conferma.
    function toggleDeleteOriginals(next: boolean) {
        if (next && !deleteOriginals) {
            setConfirmDeleteOriginals(true)
            return
        }
        applyOptions(destSameAsSource, destFolder, next)
    }

    function confirmEnableDelete() {
        setConfirmDeleteOriginals(false)
        applyOptions(destSameAsSource, destFolder, true)
    }

    function updateDraftList(
        key:
            | 'supportedExtensions'
            | 'occurrenciesToRemove'
            | 'occurrenciesToReplaceWithFt'
            | 'artistExceptions',
        text: string,
    ) {
        if (!draft) return
        setDraft({ ...draft, [key]: textToList(text) } as rules.Config)
    }

    function updateReplacement(index: number, field: 'from' | 'to', value: string) {
        if (!draft) return
        const replacements = (draft.replacements ?? []).map((r, i) =>
            i === index ? { ...r, [field]: value } : r,
        )
        setDraft({ ...draft, replacements } as rules.Config)
    }

    function addReplacement() {
        if (!draft) return
        const replacements = [...(draft.replacements ?? []), { from: '', to: '' } as rules.Replacement]
        setDraft({ ...draft, replacements } as rules.Config)
    }

    function removeReplacement(index: number) {
        if (!draft) return
        const replacements = (draft.replacements ?? []).filter((_, i) => i !== index)
        setDraft({ ...draft, replacements } as rules.Config)
    }

    // Playlist YouTube (Impostazioni): stessa logica di editing delle
    // sostituzioni Da→A, ma su un elenco a parte (playlistDraft) salvato con
    // SetPlaylists, non con SetConfig.
    function updatePlaylistDraft(index: number, field: 'name' | 'url', value: string) {
        setPlaylistDraft((prev) => prev.map((p, i) => (i === index ? { ...p, [field]: value } : p)))
    }

    function addPlaylistDraft() {
        setPlaylistDraft((prev) => [...prev, { name: '', url: '' }])
    }

    function removePlaylistDraft(index: number) {
        setPlaylistDraft((prev) => prev.filter((_, i) => i !== index))
    }

    // saveSettingsCore persiste in un colpo solo TUTTE le impostazioni della
    // schermata: prima le regole di rinomina (SetConfig, che riscansiona con le
    // nuove regole) e poi le playlist (SetPlaylists). Cattura i due draft prima di
    // qualsiasi absorb() intermedio: absorb reimposta draft/playlistDraft dallo
    // stato del server, quindi assorbiamo solo alla fine per non azzerare l'uno
    // mentre salviamo l'altro.
    async function saveSettingsCore() {
        const cfgDraft = draft
        const plDraft = playlistDraft
        if (cfgDraft) {
            await SetConfig(cfgDraft)
        }
        const resp = await SetPlaylists(plDraft)
        absorb(resp)
        setResults(null)
        setStatus({ message: resp.message ?? '', ok: resp.ok })
        notify(resp.ok, 'Impostazioni salvate.')
    }

    function saveSettings() {
        guard(saveSettingsCore)
    }

    function saveSettingsAndExit() {
        guard(async () => {
            await saveSettingsCore()
            setShowSettings(false)
        })
    }

    // "Indietro": se ci sono modifiche non salvate (regole o playlist) chiede
    // conferma (salva / scarta / annulla); altrimenti esce subito.
    function backFromSettings() {
        if (settingsDirty) {
            setConfirmLeaveSettings(true)
        } else {
            setShowSettings(false)
        }
    }

    // Scarta le modifiche non salvate: riporta i draft allo stato salvato e esce.
    function discardSettingsAndLeave() {
        setConfirmLeaveSettings(false)
        if (state?.config) setDraft(cloneConfig(state.config))
        setPlaylistDraft((state?.playlists ?? []).map((p) => ({ name: p.name, url: p.url })))
        setShowSettings(false)
    }

    // Salva le modifiche (NON come predefiniti) e poi esce, dal prompt di uscita.
    function saveSettingsAndLeaveFromPrompt() {
        setConfirmLeaveSettings(false)
        saveSettingsAndExit()
    }

    // Avvia il download della playlist selezionata. Se yt-dlp non è presente
    // chiede prima conferma con un popup: se la gestione automatica è attiva
    // propone solo di scaricarlo, altrimenti propone di attivarla e procedere
    // (in entrambi i casi lo scarica e poi prosegue). Se yt-dlp c'è, scarica
    // direttamente.
    function downloadPlaylist() {
        if (!selectedPlaylist) return
        if (!state?.ytDlpAvailable) {
            setConfirmInstallYtDlp(true)
            return
        }
        runDownloadPlaylist()
    }

    // Scarica i video della playlist nella cartella di partenza; la scansione
    // riparte automaticamente lato backend. Presume yt-dlp disponibile (o gestito
    // a mano): se manca, il backend risponde con un errore chiaro.
    function runDownloadPlaylist() {
        // Il download è annullabile (Cancel) e riporta l'avanzamento (canzoni
        // scaricate / totale) via gli eventi process:progress: azzeriamo il
        // contatore e mostriamo il tasto Annulla + la barra di avanzamento.
        setProgress(null)
        setCancellable(true)
        guard(async () => {
            const resp = await DownloadPlaylist(selectedPlaylist)
            setCancellable(false)
            absorb(resp)
            setResults(null)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        }).finally(() => {
            setProgress(null)
            setCancellable(false)
        })
    }

    // Conferma dal popup: se la gestione automatica non è attiva la attiva prima
    // (solo così l'app può scaricare la propria copia in %AppData%), poi scarica
    // yt-dlp e, se va a buon fine, procede col download della playlist: tutto in
    // un unico "busy".
    function confirmInstallThenDownload() {
        setConfirmInstallYtDlp(false)
        setProgress(null)
        setCancellable(true)
        guard(async () => {
            if (!ytDlpManaged) {
                const cfg = await SetYtDlpConfig(true, ytDlpPathDraft)
                absorb(cfg)
                syncOptions(cfg.state)
                if (!cfg.ok) {
                    setStatus({ message: cfg.message ?? '', ok: false })
                    notify(false, cfg.message ?? '')
                    return
                }
            }
            const inst = await InstallYtDlp()
            absorb(inst)
            syncOptions(inst.state)
            notify(inst.ok, inst.message ?? '')
            if (!inst.ok) {
                setStatus({ message: inst.message ?? '', ok: false })
                return
            }
            const resp = await DownloadPlaylist(selectedPlaylist)
            setCancellable(false)
            absorb(resp)
            setResults(null)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        }).finally(() => {
            setProgress(null)
            setCancellable(false)
        })
    }

    // Attiva/disattiva la gestione automatica di yt-dlp. In gestione automatica
    // l'app usa/aggiorna la propria copia in %AppData%; altrimenti si usa il
    // percorso personalizzato correntemente nel campo. È un semplice cambio di
    // impostazione: niente busy a tutta UI (che farebbe sembrare la checkbox
    // lenta): flippiamo subito in modo ottimistico e persistiamo in background,
    // assorbendo lo stato reale al ritorno.
    function toggleYtDlpManaged(next: boolean) {
        setYtDlpManaged(next)
        SetYtDlpConfig(next, ytDlpPathDraft)
            .then((resp) => {
                absorb(resp)
                syncOptions(resp.state)
                setStatus({ message: resp.message ?? '', ok: resp.ok })
            })
            .catch((e) => setStatus({ message: String(e), ok: false }))
    }

    // Persiste il percorso personalizzato (all'uscita dal campo): disattiva la
    // gestione automatica, dato che si sta puntando a un eseguibile scelto a mano.
    function applyYtDlpPath() {
        guard(async () => {
            const resp = await SetYtDlpConfig(false, ytDlpPathDraft)
            absorb(resp)
            syncOptions(resp.state)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        })
    }

    // Selettore file per scegliere l'eseguibile yt-dlp personalizzato; alla
    // conferma imposta il percorso e disattiva la gestione automatica.
    function browseYtDlp() {
        guard(async () => {
            const path = await ChooseYtDlpFile()
            if (!path) return
            setYtDlpPathDraft(path)
            const resp = await SetYtDlpConfig(false, path)
            absorb(resp)
            syncOptions(resp.state)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        })
    }

    // Scarica/installa yt-dlp nel percorso effettivo in uso (copia gestita in
    // %AppData% se la gestione automatica è attiva, altrimenti il percorso
    // personalizzato). Usato dal tasto di download mostrato quando yt-dlp non è
    // presente. Il backend risponde con un errore chiaro se manca un percorso.
    function installYtDlp() {
        setConfirmDownloadYtDlp(false)
        guard(async () => {
            const resp = await InstallYtDlp()
            absorb(resp)
            syncOptions(resp.state)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Rimuove la copia di yt-dlp gestita dall'app (%AppData%\RenameMusic), dopo
    // conferma. Ha senso solo in gestione automatica: in modalità manuale il file
    // è dell'utente e il backend rifiuta la rimozione.
    function uninstallYtDlp() {
        setConfirmUninstallYtDlp(false)
        guard(async () => {
            const resp = await UninstallYtDlp()
            absorb(resp)
            syncOptions(resp.state)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Scorciatoie da tastiera. Il gestore è tenuto in un ref aggiornato ad ogni
    // render, così il listener (registrato una sola volta) vede sempre lo stato
    // corrente senza doversi ri-registrare ad ogni cambiamento.
    const shortcutRef = useRef<(e: KeyboardEvent) => void>(() => {})
    shortcutRef.current = (e: KeyboardEvent) => {
        // Non intercettare mentre si scrive in un campo editabile.
        const target = e.target as HTMLElement | null
        if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) {
            return
        }

        // Esc chiude, in ordine: modali aperte, poi il pannello impostazioni.
        if (e.key === 'Escape') {
            if (confirmDeleteOriginals) setConfirmDeleteOriginals(false)
            else if (confirmClearTags) setConfirmClearTags(false)
            else if (confirmInstallYtDlp) setConfirmInstallYtDlp(false)
            else if (confirmUninstallYtDlp) setConfirmUninstallYtDlp(false)
            else if (confirmDownloadYtDlp) setConfirmDownloadYtDlp(false)
            else if (confirmLeaveSettings) setConfirmLeaveSettings(false)
            else if (confirmDefault) setConfirmDefault(false)
            else if (showSettings) backFromSettings()
            return
        }

        if (!e.ctrlKey) return
        switch (e.key.toLowerCase()) {
            case 'o': // Scegli cartella di partenza
                if (busy) return
                e.preventDefault()
                chooseFolder()
                break
            case 'r': // Aggiorna scansione
                if (busy || !folder) return
                e.preventDefault()
                refresh()
                break
            case 'enter': // Converti (o, nella vista risultati, nuova scansione)
                e.preventDefault()
                if (results) {
                    if (!busy && folder) refresh()
                } else if (canProcess) {
                    process()
                }
                break
            case ',': // Mostra/nascondi impostazioni
                if (busy) return
                e.preventDefault()
                setShowSettings((v) => !v)
                break
        }
    }
    useEffect(() => {
        const handler = (e: KeyboardEvent) => shortcutRef.current(e)
        window.addEventListener('keydown', handler)
        return () => window.removeEventListener('keydown', handler)
    }, [])

    return (
        <div className="app">
            {!showSettings && (
            <header>
                <div className="header-inner">
                <h1>RenameMusic</h1>
                <div className="header-right">
                    <ShortcutsLegend />
                    {!showSettings && (
                        <>
                            <Tooltip
                                label={
                                    !folder
                                        ? 'Seleziona prima una cartella di partenza'
                                        : watchEnabled
                                          ? "Aggiornamento automatico attivo: clicca per disattivarlo. Le variazioni nella cartella aggiornano l'anteprima."
                                          : "Aggiornamento automatico disattivato: clicca per attivarlo e aggiornare l'anteprima automaticamente."
                                }
                            >
                                <button
                                    type="button"
                                    className={'watch-toggle' + (watchEnabled ? ' is-on' : '')}
                                    onClick={() => toggleWatch(!watchEnabled)}
                                    disabled={busy || !folder}
                                    aria-pressed={watchEnabled}
                                >
                                    <span className="watch-dot" aria-hidden="true" />
                                    {watchEnabled ? 'Agg. automatico attivo' : 'Agg. automatico'}
                                </button>
                            </Tooltip>
                            <div className="counters">
                                <span>{fileCount} file{showingResults ? ' elaborati' : ''}</span>
                                <span className="dot">·</span>
                                <span>{mp3Count} MP3</span>
                                {toRenameCount > 0 && (
                                    <>
                                        <span className="dot">·</span>
                                        <span className="counter-hi">
                                            {toRenameCount} {showingResults ? 'rinominati' : 'da rinominare'}
                                        </span>
                                    </>
                                )}
                                {failedCount > 0 && (
                                    <>
                                        <span className="dot">·</span>
                                        <span className="counter-err">{failedCount} errori</span>
                                    </>
                                )}
                            </div>
                            <button
                                type="button"
                                className="header-btn with-icon"
                                onClick={() => setShowSettings(true)}
                                disabled={busy}
                            >
                                <span className="btn-icon"><SettingsIcon /></span>
                                Impostazioni
                            </button>
                        </>
                    )}
                </div>
                </div>
            </header>
            )}

            <main className={showSettings ? 'settings-view' : ''}>
                <div
                    className={'busy-bar' + (busy ? ' is-active' : '')}
                    role="progressbar"
                    aria-hidden={!busy}
                    aria-label="Operazione in corso"
                />

                {!showSettings && (
                <div className="top-row">
                    <div className="top-left-head">
                        <div className="field-group">
                            <span className="field-label">Cartella di partenza</span>
                            <div className="toolbar">
                                <Tooltip label={folder} grow>
                                    <div className="folder-path">
                                        {folder || 'Nessuna cartella selezionata'}
                                    </div>
                                </Tooltip>
                                <Tooltip label="Apri la cartella in Esplora risorse">
                                    <button
                                        className="ghost with-icon"
                                        onClick={() => openFolder(folder)}
                                        disabled={busy || !folder}
                                    >
                                        <span className="btn-icon"><FolderOpenIcon /></span>
                                        Apri
                                    </button>
                                </Tooltip>
                                <button className="primary" onClick={chooseFolder} disabled={busy}>
                                    Scegli cartella
                                </button>
                            </div>
                        </div>

                        {!destSameAsSource && (
                            <div className="field-group">
                                <span className="field-label">Cartella di destinazione</span>
                                <div className="toolbar">
                                    <Tooltip label={destFolder} grow>
                                        <div className="folder-path">
                                            {destFolder || 'Nessuna destinazione selezionata'}
                                        </div>
                                    </Tooltip>
                                    <Tooltip label="Apri la cartella in Esplora risorse">
                                        <button
                                            className="ghost with-icon"
                                            onClick={() => openFolder(destFolder)}
                                            disabled={busy || !destFolder}
                                        >
                                            <span className="btn-icon"><FolderOpenIcon /></span>
                                            Apri
                                        </button>
                                    </Tooltip>
                                    <button className="primary" onClick={chooseDestination} disabled={busy}>
                                        Scegli cartella
                                    </button>
                                </div>
                            </div>
                        )}

                        <div className="options">
                            <div className="check">
                                <label className="check-label">
                                    <input
                                        type="checkbox"
                                        checked={destSameAsSource}
                                        onChange={(e) => applyOptions(e.target.checked, destFolder, deleteOriginals)}
                                        disabled={busy}
                                    />
                                    Destinazione uguale alla cartella di partenza
                                </label>
                                <InfoIcon text="Se attiva, i file convertiti vengono scritti nella stessa cartella dei file originali. Se disattivata puoi scegliere una cartella di destinazione separata." />
                            </div>

                            <div className="check">
                                <label className="check-label">
                                    <input
                                        type="checkbox"
                                        checked={deleteOriginals}
                                        onChange={(e) => toggleDeleteOriginals(e.target.checked)}
                                        disabled={busy}
                                    />
                                    Eliminazione file originali
                                </label>
                                <InfoIcon text="Quando attiva, dopo la conversione i file di partenza vengono eliminati definitivamente dal disco. Quando disattivata, i nuovi file convertiti vengono scritti senza toccare gli originali." />
                            </div>
                        </div>

                        <div className="actions">
                            <div className="download-controls">
                                <Tooltip label={playlists.length === 0 ? 'Nessuna playlist salvata: aggiungine una dalle Impostazioni' : 'Playlist da scaricare'}>
                                    <PlaylistSelect
                                        value={selectedPlaylist}
                                        options={playlists}
                                        onChange={setSelectedPlaylist}
                                        disabled={busy || playlists.length === 0}
                                    />
                                </Tooltip>
                                <Tooltip label={!folder ? 'Seleziona prima una cartella di partenza' : 'Scarica la playlist selezionata'}>
                                    <button
                                        className="accent with-icon"
                                        onClick={downloadPlaylist}
                                        disabled={busy || !selectedPlaylist || !folder}
                                    >
                                        <span className="btn-icon"><DownloadIcon /></span>
                                        Scarica
                                    </button>
                                </Tooltip>
                            </div>
                            {results ? (
                                <button className="accent with-icon" onClick={refresh} disabled={busy || !folder}>
                                    <span className="btn-icon"><RefreshIcon /></span>
                                    Avvia nuova scansione
                                </button>
                            ) : (
                                <button className="accent with-icon" onClick={process} disabled={!canProcess}>
                                    <span className="btn-icon"><ConvertIcon /></span>
                                    Converti nomi e scrivi tag
                                </button>
                            )}
                            {busy && cancellable && (
                                <button className="danger-solid with-icon" onClick={cancelOp}>
                                    <span className="btn-icon"><CloseIcon /></span>
                                    Annulla
                                </button>
                            )}
                            <Tooltip label="Cancella tutti i tag ID3 dagli MP3 della cartella">
                                <button
                                    className="ghost with-icon danger"
                                    onClick={() => setConfirmClearTags(true)}
                                    disabled={!canClearTags}
                                >
                                    <span className="btn-icon"><TagOffIcon /></span>
                                    Cancella tag
                                </button>
                            </Tooltip>
                        </div>

                        {busy && progress && progress.total > 0 && (
                            <div className="op-progress">
                                <div
                                    className="op-progress-track"
                                    role="progressbar"
                                    aria-valuemin={0}
                                    aria-valuemax={progress.total}
                                    aria-valuenow={progress.done}
                                >
                                    <div
                                        className="op-progress-fill"
                                        style={{ width: Math.round((progress.done / progress.total) * 100) + '%' }}
                                    />
                                </div>
                                <span className="op-progress-label">
                                    {progress.done} / {progress.total} completati
                                </span>
                            </div>
                        )}
                    </div>

                    <div className="activity-cell">
                    <section className="panel fade-in activity-panel">
                        <div className="panel-head">
                            <h2>
                                <span className="h2-icon"><ActivityIcon /></span>
                                Attività
                            </h2>
                            <button
                                className="ghost small with-icon"
                                onClick={clearLogs}
                                disabled={busy || logs.length === 0}
                            >
                                <span className="btn-icon"><TrashIcon /></span>
                                Pulisci
                            </button>
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

                    <div className={'status ' + (status.message ? (status.ok ? 'ok' : 'err') : '')}>
                        {busy && progress
                            ? `Operazione in corso... (${progress.done}/${progress.total})`
                            : status.message}
                    </div>
                </div>
                )}

                {showSettings && draft && (
                    <>
                        <section className="settings">
                        <h2>Download da YouTube</h2>

                        <div className="check ytdlp-toggle">
                            <label className="check-label">
                                <input
                                    type="checkbox"
                                    checked={ytDlpManaged}
                                    onChange={(e) => toggleYtDlpManaged(e.target.checked)}
                                    disabled={busy}
                                />
                                Gestisci autonomamente yt-dlp
                            </label>
                            <InfoIcon text="Quando attivo, l'app scarica e aggiorna da sé yt-dlp in %AppData%\RenameMusic (scrivibile senza permessi di amministratore): al primo 'Scarica' di una playlist, se manca, lo scarica dopo una conferma. Quando disattivo, indichi a mano il percorso di una tua versione di yt-dlp." />
                        </div>

                        <div className="ytdlp-panel">
                            <div className="ytdlp-head">
                                <span className="ytdlp-title">yt-dlp</span>
                                {state?.ytDlpAvailable ? (
                                    <span className="ytdlp-badge ytdlp-ok">
                                        Presente{state?.ytDlpVersion ? ` · versione ${state.ytDlpVersion}` : ''}
                                    </span>
                                ) : (
                                    <span className="ytdlp-badge ytdlp-missing">Non presente</span>
                                )}
                                {!state?.ytDlpAvailable ? (
                                    <Tooltip label="Scarica yt-dlp">
                                        <button
                                            className="ghost small ytdlp-install"
                                            onClick={() => setConfirmDownloadYtDlp(true)}
                                            disabled={busy}
                                            aria-label="Scarica yt-dlp"
                                        >
                                            <DownloadIcon />
                                        </button>
                                    </Tooltip>
                                ) : ytDlpManaged ? (
                                    <Tooltip label="Rimuovi yt-dlp (elimina la copia gestita dall'app)">
                                        <button
                                            className="ghost small danger ytdlp-uninstall"
                                            onClick={() => setConfirmUninstallYtDlp(true)}
                                            disabled={busy}
                                            aria-label="Rimuovi yt-dlp"
                                        >
                                            <RemoveIcon />
                                        </button>
                                    </Tooltip>
                                ) : null}
                            </div>

                            <div className="ytdlp-row">
                                {ytDlpManaged ? (
                                    <Tooltip label={state?.ytDlpEffectivePath || ''} grow>
                                        <code className="ytdlp-path">
                                            {state?.ytDlpEffectivePath || '—'}
                                        </code>
                                    </Tooltip>
                                ) : (
                                    <>
                                    <span className="ytdlp-label">Percorso</span>
                                    <div className="ytdlp-path-edit">
                                        <input
                                            type="text"
                                            placeholder="Percorso a yt-dlp.exe"
                                            value={ytDlpPathDraft}
                                            onChange={(e) => setYtDlpPathDraft(e.target.value)}
                                            onBlur={applyYtDlpPath}
                                            disabled={busy}
                                        />
                                        <button className="ghost with-icon" onClick={browseYtDlp} disabled={busy}>
                                            <span className="btn-icon"><FolderOpenIcon /></span>
                                            Sfoglia
                                        </button>
                                    </div>
                                    </>
                                )}
                            </div>
                        </div>

                        <div className="replacements">
                            <div className="replacements-head">
                                <span>Playlist YouTube (nome → link)</span>
                                <button className="ghost small add-replacement" onClick={addPlaylistDraft} disabled={busy}>
                                    + Aggiungi
                                </button>
                            </div>
                            {playlistDraft.map((p, i) => (
                                <div className="replacement-row" key={i}>
                                    <input
                                        type="text"
                                        placeholder="Nome"
                                        value={p.name}
                                        onChange={(e) => updatePlaylistDraft(i, 'name', e.target.value)}
                                        disabled={busy}
                                    />
                                    <span className="arrow">→</span>
                                    <input
                                        type="text"
                                        placeholder="Link playlist"
                                        value={p.url}
                                        onChange={(e) => updatePlaylistDraft(i, 'url', e.target.value)}
                                        disabled={busy}
                                    />
                                    <button
                                        className="ghost small danger"
                                        onClick={() => removePlaylistDraft(i)}
                                        disabled={busy}
                                    >
                                        ✕
                                    </button>
                                </div>
                            ))}
                        </div>
                        </section>

                        <section className="settings">
                        <h2>Regole di rinomina (salvate su disco)</h2>

                        <div className="settings-grid">
                            <label>
                                <span>Estensioni supportate (una per riga)</span>
                                <textarea
                                    rows={6}
                                    value={listToText(draft.supportedExtensions)}
                                    onChange={(e) => updateDraftList('supportedExtensions', e.target.value)}
                                    disabled={busy}
                                />
                            </label>
                            <label>
                                <span>Occorrenze da rimuovere (una per riga)</span>
                                <textarea
                                    rows={6}
                                    value={listToText(draft.occurrenciesToRemove)}
                                    onChange={(e) => updateDraftList('occurrenciesToRemove', e.target.value)}
                                    disabled={busy}
                                />
                            </label>
                            <label>
                                <span>Alias di "ft" (una per riga)</span>
                                <textarea
                                    rows={6}
                                    value={listToText(draft.occurrenciesToReplaceWithFt)}
                                    onChange={(e) =>
                                        updateDraftList('occurrenciesToReplaceWithFt', e.target.value)
                                    }
                                    disabled={busy}
                                />
                            </label>
                            <label>
                                <span>Nomi d'arte da non separare (una per riga)</span>
                                <textarea
                                    rows={6}
                                    value={listToText(draft.artistExceptions)}
                                    onChange={(e) => updateDraftList('artistExceptions', e.target.value)}
                                    disabled={busy}
                                />
                            </label>
                        </div>

                        <div className="replacements">
                            <div className="replacements-head">
                                <span>Sostituzioni (Da → A)</span>
                                <button className="ghost small add-replacement" onClick={addReplacement} disabled={busy}>
                                    + Aggiungi
                                </button>
                            </div>
                            {(draft.replacements ?? []).map((r, i) => (
                                <div className="replacement-row" key={i}>
                                    <input
                                        type="text"
                                        placeholder="Da"
                                        value={r.from}
                                        onChange={(e) => updateReplacement(i, 'from', e.target.value)}
                                        disabled={busy}
                                    />
                                    <span className="arrow">→</span>
                                    <input
                                        type="text"
                                        placeholder="A"
                                        value={r.to}
                                        onChange={(e) => updateReplacement(i, 'to', e.target.value)}
                                        disabled={busy}
                                    />
                                    <button
                                        className="ghost small danger"
                                        onClick={() => removeReplacement(i)}
                                        disabled={busy}
                                    >
                                        ✕
                                    </button>
                                </div>
                            ))}
                        </div>
                    </section>
                    </>
                )}

                {!showSettings && (
                <section className="panel fade-in">
                    <div className="panel-head">
                        <h2>
                            <span className="h2-icon">{results ? <ConvertIcon /> : <EyeIcon />}</span>
                            {results ? 'Risultato conversione' : 'Anteprima'}
                        </h2>
                            {!results && (
                                <div className="preview-tools">
                                    <Tooltip label="Mostra solo i file che subiranno una modifica, nel nome o nei tag. È solo una vista: l'elaborazione tratta comunque tutti i file.">
                                        <label className="toggle-changed">
                                            <input
                                                type="checkbox"
                                                checked={showOnlyChanged}
                                                onChange={(e) => setShowOnlyChanged(e.target.checked)}
                                                disabled={busy}
                                            />
                                            Solo da modificare
                                        </label>
                                    </Tooltip>
                                    <Tooltip label="Aggiorna la scansione della cartella">
                                        <button
                                            className="ghost small with-icon"
                                            onClick={refresh}
                                            disabled={busy || !folder}
                                        >
                                            <span className="btn-icon"><RefreshIcon /></span>
                                            Aggiorna
                                        </button>
                                    </Tooltip>
                                </div>
                            )}
                        </div>
                        {results ? (
                            results.length === 0 ? (
                                <div className="empty">Nessun file elaborato.</div>
                            ) : (
                                <table>
                                    <thead>
                                        <tr>
                                            <th>Nome originale</th>
                                            <th>Nuovo nome</th>
                                            <th>Titolo</th>
                                            <th>Artista</th>
                                            <th>Esito</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {results.map((r, i) => {
                                            const src = splitName(r.oldName)
                                            const dst = splitName(r.newName)
                                            const renamed = !r.skipped && !r.failed && !r.canceled && src.base !== dst.base
                                            // Titolo/Artista scritti nei tag: mostrati solo per gli MP3
                                            // effettivamente elaborati (non saltati/annullati), come nell'anteprima.
                                            const showTags = r.mp3 && !r.skipped && !r.canceled
                                            const rowClass = r.failed
                                                ? 'failed'
                                                : r.canceled
                                                  ? 'skipped'
                                                  : r.skipped
                                                    ? 'skipped'
                                                    : renamed
                                                      ? 'changed'
                                                      : ''
                                            return (
                                                <tr key={i} className={rowClass}>
                                                    <td>
                                                        {renamed ? <s className="old-name">{src.base}</s> : src.base}
                                                    </td>
                                                    <td>{r.skipped || r.canceled ? '—' : dst.base}</td>
                                                    <td>
                                                        {showTags ? r.title : <span className="muted-dash">—</span>}
                                                    </td>
                                                    <td>
                                                        {showTags ? r.artist : <span className="muted-dash">—</span>}
                                                    </td>
                                                    <td>
                                                        {r.failed ? (
                                                            <ErrorLabel message={r.reason} />
                                                        ) : r.canceled ? (
                                                            <span className="badge badge-neutral">Annullato</span>
                                                        ) : r.skipped ? (
                                                            <span className="note">Saltato: {r.reason}</span>
                                                        ) : (
                                                            <div className="badges">
                                                                {renamed ? (
                                                                    <span className="badge badge-changed">Rinominato</span>
                                                                ) : (
                                                                    <span className="badge badge-neutral">Invariato</span>
                                                                )}
                                                                {r.tagged && (
                                                                    <span className="badge badge-tag">Taggato</span>
                                                                )}
                                                                <ExtChip ext={dst.ext} />
                                                            </div>
                                                        )}
                                                    </td>
                                                </tr>
                                            )
                                        })}
                                    </tbody>
                                </table>
                            )
                        ) : !booted ? (
                            <div className="empty">Caricamento…</div>
                        ) : files.length === 0 ? (
                            <div className="empty">Scegli una cartella per vedere l'anteprima.</div>
                        ) : previewFiles.length === 0 ? (
                            <div className="empty">Nessun file da modificare.</div>
                        ) : (
                            <table className="preview-table">
                                <thead>
                                    <tr>
                                        <th>File attuale</th>
                                        <th>Anteprima nuovo nome</th>
                                        <th>Anteprima nuovo titolo</th>
                                        <th>Anteprima nuovo artista</th>
                                        <th>Stato</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {previewFiles.map((file, i) => {
                                        const src = splitName(file.name)
                                        const dst = splitName(file.preview)
                                        const nameChanged = src.base !== dst.base
                                        const titleChanged = file.mp3 && tagChanged(file.title, file.titlePreview)
                                        const artistChanged = file.mp3 && tagChanged(file.artist, file.artistPreview)
                                        const rowChanged = nameChanged || titleChanged || artistChanged
                                        return (
                                            <tr key={i} className={rowChanged ? 'changed' : ''}>
                                                <td>
                                                    <CurrentField label="nome" value={src.base} changed={nameChanged} />
                                                    {file.mp3 && (
                                                        <CurrentField label="titolo" value={file.title ?? ''} changed={titleChanged} />
                                                    )}
                                                    {file.mp3 && (
                                                        <CurrentField label="artista" value={file.artist ?? ''} changed={artistChanged} />
                                                    )}
                                                </td>
                                                <td className={nameChanged ? 'value-changed' : ''}>{dst.base}</td>
                                                <td className={titleChanged ? 'value-changed' : ''}>
                                                    {file.mp3 ? file.titlePreview : <span className="muted-dash">—</span>}
                                                </td>
                                                <td className={artistChanged ? 'value-changed' : ''}>
                                                    {file.mp3 ? file.artistPreview : <span className="muted-dash">—</span>}
                                                </td>
                                                <td>
                                                    <div className="badges">
                                                        {nameChanged ? (
                                                            <span className="badge badge-changed">Da rinominare</span>
                                                        ) : (
                                                            <span className="badge badge-neutral">Invariato</span>
                                                        )}
                                                        {(titleChanged || artistChanged) && (
                                                            <span className="badge badge-tag">Da taggare</span>
                                                        )}
                                                        <ExtChip ext={src.ext} />
                                                    </div>
                                                </td>
                                            </tr>
                                        )
                                    })}
                                </tbody>
                            </table>
                        )}
                    </section>
                )}
            </main>

            {showSettings && draft && (
                <div className="settings-bottombar">
                    <div className="settings-bottombar-inner">
                        <button className="ghost with-icon settings-back" onClick={backFromSettings} disabled={busy}>
                            <span className="btn-icon"><BackIcon /></span>
                            Indietro
                        </button>
                        <div className="settings-bottombar-actions">
                            <button onClick={revertDraft} disabled={busy || !settingsDirty}>
                                Annulla modifiche
                            </button>
                            <button onClick={resetConfig} disabled={busy}>
                                Ripristina predefiniti
                            </button>
                            <span className="settings-actions-sep" aria-hidden="true" />
                            <button className="warn-solid" onClick={() => setConfirmDefault(true)} disabled={busy}>
                                Salva come predefinito
                            </button>
                            <span className="settings-actions-sep" aria-hidden="true" />
                            <button className="primary" onClick={saveSettings} disabled={busy}>
                                Salva
                            </button>
                            <button className="accent" onClick={saveSettingsAndExit} disabled={busy}>
                                Salva ed esci
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {confirmDeleteOriginals && (
                <div className="modal-overlay" onClick={() => setConfirmDeleteOriginals(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Attivare l'eliminazione degli originali?</h3>
                        <p>
                            Con questa opzione attiva, dopo ogni conversione i file originali
                            verranno <strong>eliminati definitivamente</strong>. Verifica di avere
                            un backup se ti serve poter tornare indietro.
                        </p>
                        <div className="modal-actions">
                            <button onClick={() => setConfirmDeleteOriginals(false)} disabled={busy}>
                                Annulla
                            </button>
                            <button className="danger-solid" onClick={confirmEnableDelete} disabled={busy}>
                                Continua
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {confirmClearTags && (
                <div className="modal-overlay" onClick={() => setConfirmClearTags(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Cancellare tutti i tag?</h3>
                        <p>
                            Verranno <strong>rimossi tutti i tag ID3</strong> (titolo, artista, ecc.)
                            da tutti gli MP3 della cartella di partenza. I file non vengono rinominati
                            né spostati, ma i metadati eliminati <strong>non sono recuperabili</strong>.
                        </p>
                        <div className="modal-actions">
                            <button onClick={() => setConfirmClearTags(false)} disabled={busy}>
                                Annulla
                            </button>
                            <button className="danger-solid" onClick={confirmClearTagsAction} disabled={busy}>
                                Cancella tag
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {confirmInstallYtDlp && (
                <div className="modal-overlay" onClick={() => setConfirmInstallYtDlp(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        {ytDlpManaged ? (
                            <>
                                <h3>Scaricare yt-dlp?</h3>
                                <p>
                                    yt-dlp non è presente. L'app lo scaricherà in{' '}
                                    <code>%AppData%\RenameMusic</code> e avvierà subito il download
                                    della playlist.
                                </p>
                            </>
                        ) : (
                            <>
                                <h3>Attivare la gestione automatica di yt-dlp?</h3>
                                <p>
                                    <strong>"Gestisci autonomamente"</strong> non è attivo e yt-dlp
                                    non è disponibile. Vuoi attivarlo e procedere? L'app scaricherà la
                                    propria copia in <code>%AppData%\RenameMusic</code> e avvierà
                                    subito il download della playlist.
                                </p>
                            </>
                        )}
                        <div className="modal-actions">
                            <button onClick={() => setConfirmInstallYtDlp(false)} disabled={busy}>
                                Annulla
                            </button>
                            <button className="accent" onClick={confirmInstallThenDownload} disabled={busy}>
                                {ytDlpManaged ? 'Scarica e continua' : 'Attiva e continua'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {confirmUninstallYtDlp && (
                <div className="modal-overlay" onClick={() => setConfirmUninstallYtDlp(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Disinstallare yt-dlp?</h3>
                        <p>
                            La copia gestita dall'app in <code>%AppData%\RenameMusic</code> verrà
                            <strong> rimossa</strong>. Potrai riscaricarla in qualsiasi momento dal
                            prossimo "Scarica" di una playlist.
                        </p>
                        <div className="modal-actions">
                            <button onClick={() => setConfirmUninstallYtDlp(false)} disabled={busy}>
                                Annulla
                            </button>
                            <button className="danger-solid" onClick={uninstallYtDlp} disabled={busy}>
                                Disinstalla
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {confirmDownloadYtDlp && (
                <div className="modal-overlay" onClick={() => setConfirmDownloadYtDlp(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Scaricare yt-dlp?</h3>
                        <p>
                            Verrà scaricata l'ultima versione ufficiale di <strong>yt-dlp</strong> da
                            Internet (GitHub){ytDlpManaged ? (
                                <> in <code>%AppData%\RenameMusic</code></>
                            ) : (
                                <> nel percorso indicato</>
                            )}. Assicurati di scaricarlo solo da una fonte di cui ti fidi.
                        </p>
                        <div className="modal-actions">
                            <button onClick={() => setConfirmDownloadYtDlp(false)} disabled={busy}>
                                Annulla
                            </button>
                            <button className="accent" onClick={installYtDlp} disabled={busy}>
                                Scarica
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {confirmLeaveSettings && (
                <div className="modal-overlay" onClick={() => setConfirmLeaveSettings(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Uscire dalle impostazioni?</h3>
                        <p>
                            Ci sono <strong>modifiche non salvate</strong>. Vuoi salvarle prima di
                            tornare indietro oppure scartarle?
                        </p>
                        <div className="modal-actions">
                            <button onClick={() => setConfirmLeaveSettings(false)} disabled={busy}>
                                Annulla
                            </button>
                            <button className="danger" onClick={discardSettingsAndLeave} disabled={busy}>
                                Scarta modifiche
                            </button>
                            <button className="primary" onClick={saveSettingsAndLeaveFromPrompt} disabled={busy}>
                                Salva ed esci
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {confirmDefault && (
                <div className="modal-overlay" onClick={() => setConfirmDefault(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Rendere queste regole il nuovo predefinito?</h3>
                        <p>
                            I predefiniti attuali verranno <strong>sovrascritti</strong> con le regole
                            correnti e salvati su disco. "Ripristina predefiniti" userà d'ora in poi queste
                            regole.
                        </p>
                        <div className="modal-actions">
                            <button onClick={() => setConfirmDefault(false)} disabled={busy}>
                                Annulla
                            </button>
                            <button className="accent" onClick={confirmMakeDefault} disabled={busy}>
                                Conferma
                            </button>
                        </div>
                    </div>
                </div>
            )}

            <div className="toast-container" aria-live="polite" aria-atomic="false">
                {toasts.map((t) => (
                    <div key={t.id} className={'toast ' + (t.ok ? 'toast-ok' : 'toast-err')} role="status">
                        <span className="toast-icon" aria-hidden="true">
                            {t.ok ? <CheckIcon /> : <AlertIcon />}
                        </span>
                        <span className="toast-msg">{t.message}</span>
                        <button
                            type="button"
                            className="toast-close"
                            aria-label="Chiudi notifica"
                            onClick={() => dismissToast(t.id)}
                        >
                            <CloseIcon />
                        </button>
                    </div>
                ))}
            </div>
        </div>
    )
}

export default App
