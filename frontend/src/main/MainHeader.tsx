import { isAndroid, type core } from '../api'
import { Tooltip } from '../components/controls'
import { ActivityMenu, ShortcutsLegend } from '../components/HeaderMenus'
import { SettingsIcon } from '../icons'

// Counters: contatori dell'header. Dopo un'elaborazione (showingResults)
// riassumono i risultati, altrimenti l'anteprima.
export type Counters = {
    showingResults: boolean
    fileCount: number
    mp3Count: number
    reviewCount: number
    toRenameCount: number
    failedCount: number
}

// MainHeader: header della schermata principale, con l'aggiornamento
// automatico, i contatori, il registro Attività (activity) e le Impostazioni
// (updateAvailable accende il pallino dell'aggiornamento).
export function MainHeader({
    simpleMode,
    separateDest,
    busy,
    folder,
    folderMissing,
    foldersHint,
    watchEnabled,
    onToggleWatch,
    counters,
    activity,
    updateAvailable,
    onOpenSettings,
}: {
    simpleMode: boolean
    separateDest: boolean
    busy: boolean
    folder: string
    folderMissing: boolean
    foldersHint: string
    watchEnabled: boolean
    onToggleWatch: () => void
    counters: Counters
    activity: {
        open: boolean
        onOpenChange: (open: boolean) => void
        logs: core.LogEntry[]
        unseen: 'none' | 'info' | 'error'
        onClear: () => void
    }
    updateAvailable: boolean
    onOpenSettings: () => void
}) {
    const { showingResults, fileCount, mp3Count, reviewCount, toRenameCount, failedCount } = counters
    return (
        <header>
            <div className="header-inner">
                <h1>RenameMusic</h1>
                <div className="header-right">
                    {!isAndroid && <ShortcutsLegend simple={simpleMode} separateDest={separateDest} />}
                    {/* Senza anteprima l'aggiornamento automatico non ha
                    nulla da aggiornare: in modalità semplificata il
                    toggle sparisce (e il core ne ignora gli eventi). */}
                    {!simpleMode && (
                        <Tooltip
                            label={
                                !folder || folderMissing
                                    ? foldersHint
                                    : watchEnabled
                                      ? "Aggiornamento automatico attivo: clicca per disattivarlo. Le variazioni nella cartella aggiornano l'anteprima."
                                      : "Aggiornamento automatico disattivato: clicca per attivarlo e aggiornare l'anteprima automaticamente."
                            }
                        >
                            <button
                                type="button"
                                className={
                                    'watch-toggle' + (watchEnabled ? (folderMissing ? ' is-error' : ' is-on') : '')
                                }
                                onClick={onToggleWatch}
                                disabled={busy || !folder || folderMissing}
                                aria-pressed={watchEnabled}
                            >
                                <span className="watch-dot" aria-hidden="true" />
                                {watchEnabled ? 'Agg. automatico attivo' : 'Agg. automatico'}
                            </button>
                        </Tooltip>
                    )}
                    {/* In modalità semplificata i contatori riassumono solo
                    l'ultima conversione (non c'è un'anteprima da contare). */}
                    {(!simpleMode || showingResults) && (
                        <div className="counters">
                            <span>
                                {fileCount} file{showingResults ? ' elaborati' : ''}
                            </span>
                            <span className="dot">·</span>
                            <span>{mp3Count} MP3</span>
                            {!showingResults && reviewCount > 0 && (
                                <>
                                    <span className="dot">·</span>
                                    <span className="counter-review">{reviewCount} da rivedere</span>
                                </>
                            )}
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
                    )}
                    {/* Registro Attività (non in modalità semplificata,
                    dove gli esiti arrivano solo dai toast). */}
                    {!simpleMode && (
                        <ActivityMenu
                            open={activity.open}
                            onOpenChange={activity.onOpenChange}
                            logs={activity.logs}
                            unseen={activity.unseen}
                            onClear={activity.onClear}
                            clearDisabled={busy}
                        />
                    )}
                    <button
                        type="button"
                        className="header-btn with-icon"
                        onClick={onOpenSettings}
                        disabled={busy}
                        aria-label={updateAvailable ? 'Impostazioni (aggiornamento disponibile)' : 'Impostazioni'}
                    >
                        <span className="btn-icon">
                            <SettingsIcon />
                        </span>
                        <span className="btn-label">Impostazioni</span>
                        {updateAvailable && <span className="update-dot" aria-hidden="true" />}
                    </button>
                </div>
            </div>
        </header>
    )
}

// StorageBanner (solo Android): manca l'accesso a tutti i file, senza il
// quale l'app non può leggere né rinominare i brani.
export function StorageBanner({ onRequest }: { onRequest: () => void }) {
    return (
        <div className="storage-banner" role="alert">
            <div className="storage-banner-text">
                <strong>Serve l'accesso ai file</strong>
                <span>
                    Per leggere, rinominare e scaricare i brani l'app deve poter accedere alle cartelle della memoria.
                    Attiva "Consenti l'accesso per gestire tutti i file" nella schermata che si apre, poi torna qui.
                </span>
            </div>
            <button className="accent" onClick={onRequest}>
                Concedi accesso
            </button>
        </div>
    )
}
