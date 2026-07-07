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

// LogKind classifica il tipo di riga log in base al contenuto: serve solo a
// scegliere un colore/icona nel pannello Attività.
type LogKind = 'error' | 'watch' | 'success' | 'info'

function listToText(list: string[] | undefined): string {
    return (list ?? []).join('\n')
}

function textToList(text: string): string[] {
    return text.split('\n')
}

// parseLogEntry separa il timestamp (aggiunto dal backend, formato HH:MM:SS +
// due spazi) dal messaggio e ne deduce la categoria dal contenuto testuale.
function parseLogEntry(entry: string): { time: string; message: string; kind: LogKind } {
    const m = entry.match(/^(\d{2}:\d{2}:\d{2})\s{2}(.*)$/)
    const time = m ? m[1] : ''
    const message = m ? m[2] : entry
    const lower = message.toLowerCase()
    let kind: LogKind = 'info'
    if (lower.includes('errore') || lower.includes('fallit') || lower.includes('impossibile')) {
        kind = 'error'
    } else if (lower.includes('automatica') || lower.startsWith('watch') || lower.includes('modalità watch')) {
        kind = 'watch'
    } else if (lower.startsWith('elaborati') || lower.includes('salvat')) {
        kind = 'success'
    }
    return { time, message, kind }
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

    function absorb(resp: main.ActionResponse) {
        setState(resp.state)
        if (resp.state.config) {
            setDraft(cloneConfig(resp.state.config))
        }
    }

    async function guard(fn: () => Promise<void>) {
        setBusy(true)
        setStatus({ message: 'Operazione in corso...', ok: true })
        try {
            await fn()
        } catch (err: any) {
            setStatus({ message: 'Errore: ' + (err?.message ?? String(err)), ok: false })
        } finally {
            setBusy(false)
        }
    }

    function syncOptions(s: main.StateResponse) {
        setDestSameAsSource(s.destinationSameAsSource)
        setDestFolder(s.destinationFolder ?? '')
        setDeleteOriginals(s.deleteOriginals)
        setWatchEnabled(s.watchEnabled)
    }

    // Carica lo stato iniziale (cartella + opzioni persistite); se una cartella è
    // ricordata, ne mostra subito l'anteprima.
    // Il ref evita la doppia esecuzione indotta da React.StrictMode in dev,
    // così vediamo un unico "Scansione completata" come in produzione.
    const bootedRef = useRef(false)
    useEffect(() => {
        if (bootedRef.current) return
        bootedRef.current = true
        guard(async () => {
            const resp = await GetState()
            absorb(resp)
            syncOptions(resp.state)
            if (resp.state.folder) {
                const scanned = await Scan()
                absorb(scanned)
                setStatus({ message: scanned.message ?? '', ok: scanned.ok })
            } else {
                setStatus({ message: resp.message ?? '', ok: resp.ok })
            }
        })
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
    const mp3Count = files.filter((f) => f.mp3).length
    const destReady = destSameAsSource || destFolder !== ''
    const canProcess = !busy && folder !== '' && files.length > 0 && destReady

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
                        <span className="watch-pill" title="Modalità watch attiva: variazioni nella cartella aggiornano l'anteprima">
                            <span className="watch-dot" aria-hidden="true" />
                            Watch attivo
                        </span>
                    )}
                    <div className="counters">
                        <span>{files.length} file</span>
                        <span className="dot">·</span>
                        <span>{mp3Count} MP3</span>
                    </div>
                </div>
            </header>

            <main>
                <div className="toolbar">
                    <div className="folder-path" title={folder}>
                        {folder || 'Nessuna cartella selezionata'}
                    </div>
                    <button
                        className="icon-btn"
                        onClick={refresh}
                        disabled={busy || !folder}
                        title="Aggiorna scansione della cartella"
                        aria-label="Aggiorna scansione"
                    >
                        <RefreshIcon />
                    </button>
                    <button className="primary" onClick={chooseFolder} disabled={busy}>
                        Scegli cartella
                    </button>
                </div>

                <div className="options">
                    <label className="check">
                        <input
                            type="checkbox"
                            checked={destSameAsSource}
                            onChange={(e) => applyOptions(e.target.checked, destFolder, deleteOriginals)}
                            disabled={busy}
                        />
                        Destinazione uguale alla cartella di partenza
                    </label>

                    {!destSameAsSource && (
                        <div className="toolbar">
                            <div className="folder-path" title={destFolder}>
                                {destFolder || 'Nessuna destinazione selezionata'}
                            </div>
                            <button className="primary" onClick={chooseDestination} disabled={busy}>
                                Scegli destinazione
                            </button>
                        </div>
                    )}

                    <label className="check">
                        <input
                            type="checkbox"
                            checked={deleteOriginals}
                            onChange={(e) => applyOptions(destSameAsSource, destFolder, e.target.checked)}
                            disabled={busy}
                        />
                        Elimina file originali
                        <span className="hint">
                            {deleteOriginals
                                ? '(gli originali vengono eliminati dopo la scrittura)'
                                : '(scrive i nuovi file lasciando intatti gli originali)'}
                        </span>
                    </label>

                    <label className="check">
                        <input
                            type="checkbox"
                            checked={watchEnabled}
                            onChange={(e) => toggleWatch(e.target.checked)}
                            disabled={busy || !folder}
                        />
                        Modalità watch (aggiorna l'anteprima automaticamente)
                        <span className="hint">
                            {watchEnabled
                                ? '(le variazioni nella cartella aggiornano l\'anteprima; la conversione resta manuale)'
                                : '(disattivata: l\'anteprima si aggiorna solo con "Scegli cartella" o "Aggiorna")'}
                        </span>
                    </label>
                </div>

                <div className="actions">
                    {results ? (
                        <button className="accent" onClick={refresh} disabled={busy || !folder}>
                            Avvia nuova scansione
                        </button>
                    ) : (
                        <button className="accent" onClick={process} disabled={!canProcess}>
                            Converti nomi e scrivi tag
                        </button>
                    )}
                    <button className="ghost" onClick={() => setShowSettings((v) => !v)} disabled={busy}>
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
                                <button className="ghost small" onClick={addReplacement} disabled={busy}>
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
                            <button className="primary" onClick={saveConfig} disabled={busy}>
                                Salva impostazioni
                            </button>
                            <button onClick={revertDraft} disabled={busy}>
                                Ripristina precedenti
                            </button>
                            <button onClick={resetConfig} disabled={busy}>
                                Ripristina default
                            </button>
                            <button
                                className="accent"
                                onClick={() => setConfirmDefault(true)}
                                disabled={busy}
                            >
                                Rendi queste il default
                            </button>
                        </div>
                    </section>
                )}

                <div className="grid">
                    <section className="panel">
                        <h2>{results ? 'Risultato conversione' : 'Anteprima'}</h2>
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
                                            const renamed = !r.skipped && r.oldName !== r.newName
                                            const rowClass = r.skipped ? 'skipped' : renamed ? 'changed' : ''
                                            return (
                                                <tr key={i} className={rowClass}>
                                                    <td>{r.oldName}</td>
                                                    <td>{r.skipped ? '—' : r.newName}</td>
                                                    <td>
                                                        {r.skipped ? (
                                                            <span className="note">Saltato: {r.reason}</span>
                                                        ) : r.tagged ? (
                                                            <span className="badge badge-mp3">MP3</span>
                                                        ) : (
                                                            <span className="note">OK</span>
                                                        )}
                                                    </td>
                                                </tr>
                                            )
                                        })}
                                    </tbody>
                                </table>
                            )
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
                                        const changed = file.preview !== file.name
                                        return (
                                            <tr key={i} className={changed ? 'changed' : ''}>
                                                <td>{file.name}</td>
                                                <td>{file.preview}</td>
                                                <td>
                                                    <div className="badges">
                                                        {changed ? (
                                                            <span className="badge badge-changed">Da rinominare</span>
                                                        ) : (
                                                            <span className="badge badge-neutral">Invariato</span>
                                                        )}
                                                        {file.mp3 && <span className="badge badge-mp3">MP3</span>}
                                                    </div>
                                                </td>
                                            </tr>
                                        )
                                    })}
                                </tbody>
                            </table>
                        )}
                    </section>

                    <section className="panel">
                        <div className="panel-head">
                            <h2>Attività</h2>
                            <button
                                className="ghost small"
                                onClick={clearLogs}
                                disabled={busy || logs.length === 0}
                            >
                                Pulisci
                            </button>
                        </div>
                        <ul className="log">
                            {logs.length === 0 ? (
                                <li className="log-empty">Nessuna attività.</li>
                            ) : (
                                logs.map((log, i) => {
                                    const { time, message, kind } = parseLogEntry(log)
                                    return (
                                        <li key={i} className={'log-item log-' + kind}>
                                            <span className="log-dot" aria-hidden="true" />
                                            {time && <span className="log-time">{time}</span>}
                                            <span className="log-msg">{message}</span>
                                        </li>
                                    )
                                })
                            )}
                        </ul>
                    </section>
                </div>
            </main>

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
