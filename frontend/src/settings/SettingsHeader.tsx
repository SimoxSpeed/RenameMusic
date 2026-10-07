import { isAndroid } from '../api'
import { Tooltip } from '../components/controls'
import { DefaultsMenu, ShortcutsLegend } from '../components/HeaderMenus'
import { BackIcon, CheckIcon } from '../icons'

// SettingsHeader: barra delle Impostazioni, al posto dell'header, fissa in
// cima. Le modifiche si salvano da sole: qui restano solo l'uscita, lo stato
// del salvataggio automatico e i predefiniti (regole, playlist e modalità
// semplificata), riuniti su Android nel menù "Predefiniti".
export function SettingsHeader({
    busy,
    saveStatus,
    onBack,
    defaultsMenuOpen,
    onDefaultsMenuChange,
    onResetDefaults,
    onSaveDefaults,
    simpleMode,
    separateDest,
}: {
    busy: boolean
    saveStatus: 'saving' | 'saved' | null
    onBack: () => void
    defaultsMenuOpen: boolean
    onDefaultsMenuChange: (open: boolean) => void
    onResetDefaults: () => void
    onSaveDefaults: () => void
    simpleMode: boolean
    separateDest: boolean
}) {
    return (
        <header className="settings-header">
            <div className="header-inner">
                <div className="settings-header-title">
                    <button
                        type="button"
                        className="header-btn settings-back"
                        onClick={onBack}
                        disabled={busy}
                        aria-label="Indietro"
                    >
                        <BackIcon />
                        <span className="btn-label">Indietro</span>
                    </button>
                    <h1>Impostazioni</h1>
                    {saveStatus && (
                        <span className="save-status" role="status">
                            {/* Su Android, per stare sulla riga con
                                "Predefiniti", sul telefono resta solo
                                l'icona (spinner o spunta, mobile.css). */}
                            {saveStatus === 'saving' ? (
                                <>
                                    {isAndroid && <span className="spinner" aria-hidden="true" />}
                                    <span className="save-status-text">Salvataggio…</span>
                                </>
                            ) : (
                                <>
                                    <CheckIcon />
                                    <span className="save-status-text">Salvato</span>
                                </>
                            )}
                        </span>
                    )}
                </div>
                <div className="settings-header-actions">
                    {isAndroid ? (
                        <DefaultsMenu
                            open={defaultsMenuOpen}
                            onOpenChange={onDefaultsMenuChange}
                            onReset={onResetDefaults}
                            onSave={onSaveDefaults}
                            disabled={busy}
                        />
                    ) : (
                        <>
                            <ShortcutsLegend simple={simpleMode} separateDest={separateDest} settings />
                            <Tooltip label="Riporta regole, playlist e modalità semplificata ai predefiniti salvati.">
                                <button type="button" className="header-btn" onClick={onResetDefaults} disabled={busy}>
                                    Ripristina predefiniti
                                </button>
                            </Tooltip>
                            <Tooltip label="I predefiniti sono una configurazione di riserva, da recuperare con «Ripristina predefiniti». Salva come predefiniti regole, playlist e modalità semplificata attuali.">
                                <button
                                    type="button"
                                    className="header-btn warn-solid"
                                    onClick={onSaveDefaults}
                                    disabled={busy}
                                >
                                    Salva predefiniti
                                </button>
                            </Tooltip>
                        </>
                    )}
                </div>
            </div>
        </header>
    )
}
