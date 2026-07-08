import { useEffect, useRef, useState } from 'react'
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
} from '../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../wailsjs/runtime/runtime'
import { main, rules } from '../wailsjs/go/models'

type Status = { message: string; ok: boolean }

function listToText(list: string[] | undefined): string {
    return (list ?? []).join('\n')
}

function textToList(text: string): string[] {
    return text.split('\n')
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

function cloneConfig(cfg: rules.Config): rules.Config {
    return {
        startFolder: cfg.startFolder,
        supportedExtensions: [...(cfg.supportedExtensions ?? [])],
        occurrenciesToRemove: [...(cfg.occurrenciesToRemove ?? [])],
        occurrenciesToReplaceWithFt: [...(cfg.occurrenciesToReplaceWithFt ?? [])],
        replacements: (cfg.replacements ?? []).map((r) => ({ from: r.from, to: r.to })),
    } as rules.Config
}

function App() {
    const [state, setState] = useState<main.StateResponse | null>(null)
    const [status, setStatus] = useState<Status>({ message: '', ok: true })
    const [busy, setBusy] = useState(false)
    const [showSettings, setShowSettings] = useState(false)
    const [draft, setDraft] = useState<rules.Config | null>(null)
    const [results, setResults] = useState<main.ResultView[] | null>(null)
    const [confirmDefault, setConfirmDefault] = useState(false)
    const [destSameAsSource, setDestSameAsSource] = useState(true)
    const [destFolder, setDestFolder] = useState('')
    const [deleteOriginals, setDeleteOriginals] = useState(false)
    const [watchEnabled, setWatchEnabled] = useState(false)
    const [confirmDeleteOriginals, setConfirmDeleteOriginals] = useState(false)
    // booted diventa true al termine del caricamento iniziale: finché è false
    // mostriamo un placeholder di caricamento invece del messaggio "vuoto",
    // così al refresh non si vede un lampo di stato vuoto prima dei dati.
    const [booted, setBooted] = useState(false)

    function absorb(resp: main.ActionResponse) {
        setState(resp.state)
        if (resp.state.config) {
            setDraft(cloneConfig(resp.state.config))
        }
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

    function syncOptions(s: main.StateResponse) {
        setDestSameAsSource(s.destinationSameAsSource)
        setDestFolder(s.destinationFolder ?? '')
        setDeleteOriginals(s.deleteOriginals)
        setWatchEnabled(s.watchEnabled)
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

    // Processo unificato: normalizzazione nomi + scrittura tag in un colpo solo.
    // Usa le opzioni persistite lato backend.
    function process() {
        if (!destSameAsSource && destFolder === '') {
            setStatus({ message: 'Scegli una cartella di destinazione o riattiva "uguale alla partenza".', ok: false })
            return
        }
        guard(async () => {
            const resp = await ProcessAll()
            absorb(resp)
            setResults(resp.results ?? [])
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        })
    }

    function saveConfig() {
        if (!draft) return
        guard(async () => {
            const resp = await SetConfig(draft)
            absorb(resp)
            setResults(null)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        })
    }

    function resetConfig() {
        guard(async () => {
            const resp = await ResetConfig()
            absorb(resp)
            setResults(null)
            setStatus({ message: resp.message ?? '', ok: resp.ok })
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

    // Riporta il draft alle regole salvate (scarta le modifiche non ancora salvate).
    function revertDraft() {
        if (!state?.config) return
        setDraft(cloneConfig(state.config))
        setStatus({ message: 'Ripristinate le regole salvate.', ok: true })
    }

    // Rende le regole attuali il nuovo default (dopo conferma dal popup).
    // Non tocca le regole correnti né il draft in editing: aggiorna solo il log/stato.
    function confirmMakeDefault() {
        if (!draft) return
        setConfirmDefault(false)
        guard(async () => {
            const resp = await SetAsDefault(draft)
            setState((prev) => (prev ? ({ ...prev, logs: resp.state.logs } as main.StateResponse) : resp.state))
            setStatus({ message: resp.message ?? '', ok: resp.ok })
        })
    }

    const folder = state?.folder ?? ''
    const files = state?.files ?? []
    const logs = state?.logs ?? []
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
        key: 'supportedExtensions' | 'occurrenciesToRemove' | 'occurrenciesToReplaceWithFt',
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

    return (
        <div className="app">
            <header>
                <h1>RenameMusic</h1>
                <div className="header-right">
                    {state?.watchActive && (
                        <span className="watch-pill" title="Aggiornamento automatico attivo: variazioni nella cartella aggiornano l'anteprima">
                            <span className="watch-dot" aria-hidden="true" />
                            Agg. automatico attivo
                        </span>
                    )}
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
                </div>
            </header>

            <main>
                <div
                    className={'busy-bar' + (busy ? ' is-active' : '')}
                    role="progressbar"
                    aria-hidden={!busy}
                    aria-label="Operazione in corso"
                />

                <div className="field-group">
                    <span className="field-label">Cartella di partenza</span>
                    <div className="toolbar">
                        <div className="folder-path" title={folder}>
                            {folder || 'Nessuna cartella selezionata'}
                        </div>
                        <button
                            className="icon-btn"
                            onClick={refresh}
                            disabled={busy || !folder}
                            aria-label="Aggiorna scansione"
                        >
                            <RefreshIcon />
                            <span className="info-tooltip" role="tooltip">Aggiorna la scansione della cartella</span>
                        </button>
                        <button className="primary" onClick={chooseFolder} disabled={busy}>
                            Scegli cartella
                        </button>
                    </div>
                </div>

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

                    {!destSameAsSource && (
                        <div className="field-group">
                            <span className="field-label">Cartella di destinazione</span>
                            <div className="toolbar">
                                <div className="folder-path" title={destFolder}>
                                    {destFolder || 'Nessuna destinazione selezionata'}
                                </div>
                                <button className="primary" onClick={chooseDestination} disabled={busy}>
                                    Scegli destinazione
                                </button>
                            </div>
                        </div>
                    )}

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

                    <div className="check">
                        <label className="check-label">
                            <input
                                type="checkbox"
                                checked={watchEnabled}
                                onChange={(e) => toggleWatch(e.target.checked)}
                                disabled={busy || !folder}
                            />
                            Aggiornamento automatico
                        </label>
                        <InfoIcon text="Quando attivo, l'applicazione osserva la cartella di partenza: se aggiungi, modifichi o rimuovi file, l'anteprima si aggiorna da sola. La conversione resta comunque manuale: nessun file viene mai rinominato senza il tuo comando esplicito. Quando disattivato, l'anteprima si aggiorna solo con 'Scegli cartella' o con il pulsante di aggiornamento." />
                    </div>
                </div>

                <div className="actions">
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
                    <button className="ghost with-icon" onClick={() => setShowSettings((v) => !v)} disabled={busy}>
                        <span className="btn-icon"><SettingsIcon /></span>
                        {showSettings ? 'Nascondi impostazioni' : 'Impostazioni'}
                    </button>
                </div>

                <div className={'status ' + (status.message ? (status.ok ? 'ok' : 'err') : '')}>
                    {status.message}
                </div>

                {showSettings && draft && (
                    <section className="settings">
                        <h2>Impostazioni regole (salvate su disco)</h2>
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

                        <div className="settings-actions">
                            <div className="settings-actions-group">
                                <button className="primary" onClick={saveConfig} disabled={busy}>
                                    Salva
                                </button>
                                <button onClick={revertDraft} disabled={busy}>
                                    Annulla modifiche
                                </button>
                                <button onClick={resetConfig} disabled={busy}>
                                    Ripristina predefiniti
                                </button>
                            </div>
                            <span className="settings-actions-sep" aria-hidden="true" />
                            <div className="settings-actions-group">
                                <button
                                    className="accent"
                                    onClick={() => setConfirmDefault(true)}
                                    disabled={busy}
                                >
                                    Salva come predefinito
                                </button>
                            </div>
                        </div>
                    </section>
                )}

                <div className="grid">
                    <section className="panel fade-in">
                        <h2>
                            <span className="h2-icon">{results ? <ConvertIcon /> : <EyeIcon />}</span>
                            {results ? 'Risultato conversione' : 'Anteprima'}
                        </h2>
                        {results ? (
                            results.length === 0 ? (
                                <div className="empty">Nessun file elaborato.</div>
                            ) : (
                                <table>
                                    <thead>
                                        <tr>
                                            <th>Nome originale</th>
                                            <th>Nuovo nome</th>
                                            <th>Esito</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {results.map((r, i) => {
                                            const src = splitName(r.oldName)
                                            const dst = splitName(r.newName)
                                            const renamed = !r.skipped && !r.failed && src.base !== dst.base
                                            const rowClass = r.failed
                                                ? 'failed'
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
                                                    <td>{r.skipped ? '—' : dst.base}</td>
                                                    <td>
                                                        {r.failed ? (
                                                            <span className="note error">Errore: {r.reason}</span>
                                                        ) : r.skipped ? (
                                                            <span className="note">Saltato: {r.reason}</span>
                                                        ) : (
                                                            <div className="badges">
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
                        ) : (
                            <table>
                                <thead>
                                    <tr>
                                        <th>File attuale</th>
                                        <th>Anteprima nuovo nome</th>
                                        <th>Stato</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {files.map((file, i) => {
                                        const src = splitName(file.name)
                                        const dst = splitName(file.preview)
                                        const changed = src.base !== dst.base
                                        return (
                                            <tr key={i} className={changed ? 'changed' : ''}>
                                                <td>
                                                    {changed ? <s className="old-name">{src.base}</s> : src.base}
                                                </td>
                                                <td>{dst.base}</td>
                                                <td>
                                                    <div className="badges">
                                                        {changed ? (
                                                            <span className="badge badge-changed">Da rinominare</span>
                                                        ) : (
                                                            <span className="badge badge-neutral">Invariato</span>
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

                    <section className="panel fade-in">
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
            </main>

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

            {confirmDefault && (
                <div className="modal-overlay" onClick={() => setConfirmDefault(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Rendere queste regole il nuovo default?</h3>
                        <p>
                            I default attuali verranno <strong>sovrascritti</strong> con le regole
                            correnti e salvati su disco. "Ripristina default" userà d'ora in poi queste
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
        </div>
    )
}

export default App
