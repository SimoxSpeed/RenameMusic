import { isAndroid, type core, type playlist } from '../api'
import { CheckOption, InfoIcon, OpProgress, Tooltip } from '../components/controls'
import { PrefsButton } from '../components/playlists'
import { AccountIcon, CloseIcon, DownloadIcon, FolderOpenIcon, RefreshIcon, RemoveIcon } from '../icons'
import { playlistKeyOf } from '../lib/links'
import { prefsSummary } from '../lib/config'
import { installLabel, installPercent, type InstallProgress } from '../lib/progress'

// Pannelli della scheda Download delle Impostazioni: yt-dlp e ffmpeg,
// account Google, playlist dell'account e playlist salvate.

// YtDlpPanel: yt-dlp (copia gestita dall'app o percorso scelto a mano; su
// Android integrato, si può solo aggiornare) e ffmpeg (solo desktop), con
// stato, download/rimozione e avanzamento del download. checking: ricerca
// della copia locale in corso, dopo aver riattivato la gestione autonoma.
export function YtDlpPanel({
    state,
    busy,
    managed,
    checking,
    onManagedChange,
    pathDraft,
    onPathDraftChange,
    onPathCommit,
    onBrowse,
    installProgress,
    onDownloadYtDlp,
    onUninstallYtDlp,
    onDownloadFFmpeg,
    onUninstallFFmpeg,
}: {
    state: core.StateResponse
    busy: boolean
    managed: boolean
    checking: boolean
    onManagedChange: (managed: boolean) => void
    pathDraft: string
    onPathDraftChange: (path: string) => void
    onPathCommit: () => void
    onBrowse: () => void
    installProgress: InstallProgress | null
    onDownloadYtDlp: () => void
    onUninstallYtDlp: () => void
    onDownloadFFmpeg: () => void
    onUninstallFFmpeg: () => void
}) {
    return (
        <>
            {/* Su Android yt-dlp è integrato nell'app: niente scelta
                tra copia gestita e percorso personalizzato. */}
            {!isAndroid && (
                <CheckOption
                    className="ytdlp-toggle"
                    label="Gestisci autonomamente yt-dlp"
                    info="Quando attivo, l'app tiene una propria copia di yt-dlp in %AppData%\RenameMusic (scrivibile senza permessi di amministratore) e la aggiorna da sola. Se manca, puoi scaricarla dal tasto accanto a «Non presente» oppure, dopo una conferma, al primo download di una playlist. Quando disattivo, indichi a mano il percorso di una tua copia di yt-dlp."
                    checked={managed}
                    onChange={onManagedChange}
                    disabled={busy || checking}
                />
            )}

            <div className="ytdlp-panel">
                <div className="ytdlp-head">
                    {/* Nome, stato (la versione, se presente) e in gestione
                        autonoma il percorso della copia dell'app: se non
                        c'è spazio il percorso va a capo. Il tasto per
                        scaricarlo resta sempre subito a destra di "Non
                        presente". */}
                    <div className="ytdlp-head-info">
                        <span className="ytdlp-title">yt-dlp</span>
                        {checking ? (
                            <span className="ytdlp-checking" role="status">
                                <span className="spinner" aria-hidden="true" />
                                Ricerca di una copia locale…
                            </span>
                        ) : state.ytDlpAvailable ? (
                            <span
                                className="ytdlp-badge ytdlp-ok"
                                title={state.ytDlpVersion ? 'Versione di yt-dlp in uso' : undefined}
                            >
                                {state.ytDlpVersion || 'Presente'}
                            </span>
                        ) : (
                            <span className="ytdlp-status">
                                <span className="ytdlp-badge ytdlp-missing">
                                    {isAndroid ? 'Non ancora pronto' : 'Non presente'}
                                </span>
                                {!isAndroid && (
                                    <Tooltip label="Scarica yt-dlp">
                                        <button
                                            className="ghost small ytdlp-install"
                                            onClick={onDownloadYtDlp}
                                            disabled={busy}
                                            aria-label="Scarica yt-dlp"
                                        >
                                            <DownloadIcon />
                                        </button>
                                    </Tooltip>
                                )}
                            </span>
                        )}
                        {managed && !checking && state.ytDlpEffectivePath && (
                            <span className="ytdlp-location">
                                {!state.ytDlpAvailable && (
                                    <>
                                        <span className="ytdlp-location-label">Verrà scaricato in:</span>{' '}
                                    </>
                                )}
                                <code className="ytdlp-path">{state.ytDlpEffectivePath}</code>
                            </span>
                        )}
                        {/* Fuori dalla gestione autonoma il percorso si imposta
                        a mano, sulla stessa riga (scende sotto se non c'è
                        spazio). */}
                        {!managed && (
                            <div className="ytdlp-path-edit">
                                <span className="ytdlp-location-label">Percorso:</span>
                                <input
                                    type="text"
                                    placeholder="Percorso a yt-dlp.exe"
                                    value={pathDraft}
                                    onChange={(e) => onPathDraftChange(e.target.value)}
                                    onBlur={onPathCommit}
                                    disabled={busy}
                                />
                                <button className="ghost with-icon" onClick={onBrowse} disabled={busy}>
                                    <span className="btn-icon">
                                        <FolderOpenIcon />
                                    </span>
                                    Sfoglia
                                </button>
                            </div>
                        )}
                    </div>
                    {checking ? null : isAndroid ? (
                        <Tooltip label="Aggiorna yt-dlp all'ultima versione (YouTube cambia spesso: se i download falliscono, aggiornalo)">
                            <button
                                className="ghost small with-icon ytdlp-install"
                                onClick={onDownloadYtDlp}
                                disabled={busy}
                            >
                                <span className="btn-icon">
                                    <RefreshIcon />
                                </span>
                                Aggiorna
                            </button>
                        </Tooltip>
                    ) : state.ytDlpAvailable && managed ? (
                        <Tooltip label="Rimuovi yt-dlp (elimina la copia gestita dall'app)">
                            <button
                                className="ghost small danger ytdlp-uninstall"
                                onClick={onUninstallYtDlp}
                                disabled={busy}
                                aria-label="Rimuovi yt-dlp"
                            >
                                <RemoveIcon />
                            </button>
                        </Tooltip>
                    ) : null}
                </div>

                {busy && installProgress?.tool === 'yt-dlp' && (
                    <OpProgress
                        className="ytdlp-progress"
                        percent={installPercent(installProgress)}
                        label={installLabel(installProgress)}
                    />
                )}

                {/* ffmpeg serve a yt-dlp per creare gli mp3. Su Android
                    è incorporato in youtubedl-android. */}
                {!isAndroid && (
                    <div className="ytdlp-head">
                        <span className="ytdlp-title">ffmpeg</span>
                        {state.ffmpegAvailable ? (
                            <>
                                {/* Si rimuove solo la copia gestita: un
                                    ffmpeg di sistema non è dell'app. */}
                                <span className="ytdlp-badge ytdlp-ok">
                                    Presente{!state.ffmpegManaged && ' · di sistema'}
                                </span>
                                {state.ffmpegManaged && (
                                    <Tooltip label="Rimuovi ffmpeg (elimina la copia gestita dall'app)">
                                        <button
                                            className="ghost small danger ytdlp-uninstall"
                                            onClick={onUninstallFFmpeg}
                                            disabled={busy}
                                            aria-label="Rimuovi ffmpeg"
                                        >
                                            <RemoveIcon />
                                        </button>
                                    </Tooltip>
                                )}
                            </>
                        ) : (
                            <>
                                <span className="ytdlp-badge ytdlp-missing">Non presente</span>
                                <Tooltip label="Scarica ffmpeg (serve a yt-dlp per creare gli mp3)">
                                    <button
                                        className="ghost small ytdlp-install"
                                        onClick={onDownloadFFmpeg}
                                        disabled={busy}
                                        aria-label="Scarica ffmpeg"
                                    >
                                        <DownloadIcon />
                                    </button>
                                </Tooltip>
                            </>
                        )}
                    </div>
                )}

                {busy && installProgress?.tool === 'ffmpeg' && (
                    <OpProgress
                        className="ytdlp-progress"
                        percent={installPercent(installProgress)}
                        label={installLabel(installProgress)}
                    />
                )}
            </div>
        </>
    )
}

// GoogleAccountPanel: account Google, che serve a leggere e modificare le
// playlist di YouTube dell'utente. Senza le credenziali OAuth (available
// falso, solo desktop) resta visibile e spiega cosa manca. signingIn: accesso
// in corso (su desktop si completa nel browser e si annulla con onCancel);
// syncing: controllo in corso delle impostazioni condivise sull'account.
export function GoogleAccountPanel({
    google,
    available,
    connected,
    signingIn,
    syncing,
    busy,
    onSignIn,
    onSignOut,
    onCancel,
}: {
    google: core.GoogleView | undefined
    available: boolean
    connected: boolean
    signingIn: boolean
    syncing: boolean
    busy: boolean
    onSignIn: () => void
    onSignOut: () => void
    onCancel: () => void
}) {
    return (
        <div className="ytdlp-panel google-panel">
            {/* Titolo con la "i" a sinistra, stato (o email
                dell'account) allineato a destra accanto
                all'azione; su Android lo stato va sotto il
                titolo e l'azione resta a destra. */}
            <div className="ytdlp-head">
                <div className="ytdlp-head-info">
                    <span className="google-title">
                        <AccountIcon />
                        Account Google
                    </span>
                    <InfoIcon text="Collegando il tuo account l'app può leggere le tue playlist di YouTube (anche quelle private) per importarle qui, mostrarle per prime nella scelta del download, aggiungere a una playlist il video del link e, secondo le impostazioni di ogni playlist (⚙), togliere da YouTube i brani appena scaricati o aggiungerli a un'altra playlist. Puoi revocare l'accesso in qualsiasi momento con «Scollega»." />
                </div>
                {!available ? (
                    <span className="ytdlp-badge google-off">Non configurato</span>
                ) : connected ? (
                    <span className="google-account-row">
                        <span className="ytdlp-badge ytdlp-ok google-account" title="Account collegato">
                            {google?.email || 'Collegato'}
                        </span>
                    </span>
                ) : (
                    <span className="ytdlp-badge google-off">Non collegato</span>
                )}
                {!available ? null : connected ? (
                    <button className="ghost small danger" onClick={onSignOut} disabled={busy}>
                        Scollega
                    </button>
                ) : signingIn && !isAndroid ? (
                    <button className="danger-solid small with-icon" onClick={onCancel}>
                        <span className="btn-icon">
                            <CloseIcon />
                        </span>
                        Annulla
                    </button>
                ) : (
                    <button className="accent small with-icon" onClick={onSignIn} disabled={busy}>
                        <span className="btn-icon">
                            <AccountIcon />
                        </span>
                        Collega
                    </button>
                )}
            </div>
            {connected && !google?.email && (
                <p className="google-hint google-sub">
                    Per vedere l'indirizzo dell'account collegato, scollegalo e ricollegalo.
                </p>
            )}
            {/* Impostazioni delle playlist condivise con gli altri
                dispositivi dello stesso account (Google Drive). */}
            {connected &&
                (syncing ? (
                    <p className="google-hint" role="status">
                        <span className="spinner" aria-hidden="true" />
                        Controllo delle impostazioni salvate sull'account…
                    </p>
                ) : (
                    <p className={'google-hint google-sub' + (google?.syncError ? ' google-sync-error' : '')}>
                        {google?.syncError ||
                            'Regole, playlist, predefiniti e impostazioni delle playlist (⚙) sono condivisi con gli altri dispositivi collegati a questo account. Cartelle e opzioni di conversione restano di ogni dispositivo.'}
                    </p>
                ))}
            {!available && (
                <p className="google-hint">
                    Questa versione dell'app non ha le credenziali OAuth di Google Cloud
                    (internal/google/credentials_local.go): ricompilala con quelle del client «App desktop» per
                    collegare l'account.
                </p>
            )}
            {signingIn && !isAndroid && (
                <p className="google-hint" role="status">
                    <span className="spinner" aria-hidden="true" />
                    Completa l'accesso nella pagina di Google che si è aperta nel browser, poi torna qui.
                </p>
            )}
        </div>
    )
}

// AccountPlaylists: playlist dell'account Google, le prime della scelta del
// download, ognuna con le sue impostazioni (⚙). emptying è l'ID di quella che
// si sta svuotando («Svuota ora»), con avanzamento (progress) e Annulla.
export function AccountPlaylists({
    playlists,
    prefs,
    emptying,
    progress,
    busy,
    onCancel,
    onOpenPrefs,
}: {
    playlists: core.GooglePlaylistView[]
    prefs: Record<string, playlist.Prefs>
    emptying: string
    progress: { done: number; total: number } | null
    busy: boolean
    onCancel: () => void
    onOpenPrefs: (key: string, title: string) => void
}) {
    return (
        <div className="replacements">
            <div className="replacements-head">
                <span>Playlist del tuo account YouTube</span>
            </div>
            {playlists.length === 0 ? (
                <p className="rule-empty">Il tuo account non ha ancora playlist.</p>
            ) : (
                <ul className="account-playlists">
                    {playlists.map((p) => {
                        const key = 'yt:' + p.id
                        const summary = prefsSummary(prefs[key])
                        return (
                            <li key={p.id} className={'account-playlist' + (prefs[key]?.hidden ? ' is-hidden' : '')}>
                                <span className="account-playlist-text">
                                    <span className="account-playlist-title">{p.title || p.id}</span>
                                    <span className="account-playlist-meta">
                                        {p.count === 1 ? '1 video' : `${p.count} video`}
                                        {summary && (
                                            <>
                                                {' '}
                                                · <span className="account-playlist-prefs">{summary}</span>
                                            </>
                                        )}
                                    </span>
                                </span>
                                {emptying === p.id ? (
                                    <button className="danger-solid small with-icon" onClick={onCancel}>
                                        <span className="btn-icon">
                                            <CloseIcon />
                                        </span>
                                        Annulla
                                    </button>
                                ) : (
                                    <PrefsButton
                                        summary={summary}
                                        onClick={() => onOpenPrefs(key, p.title || p.id)}
                                        disabled={busy}
                                    />
                                )}
                                {emptying === p.id && progress && progress.total > 0 && (
                                    <OpProgress
                                        className="account-playlist-progress"
                                        percent={Math.round((progress.done / progress.total) * 100)}
                                        label={`Svuotamento · ${progress.done} / ${progress.total} brani tolti`}
                                    />
                                )}
                            </li>
                        )
                    })}
                </ul>
            )}
        </div>
    )
}

// SavedPlaylists: playlist salvate (nome → link), modificabili riga per riga;
// con l'account collegato (connected) si possono anche importare dall'account.
export function SavedPlaylists({
    rows,
    prefs,
    connected,
    busy,
    onImport,
    onAdd,
    onChange,
    onRemove,
    onOpenPrefs,
}: {
    rows: playlist.Playlist[]
    prefs: Record<string, playlist.Prefs>
    connected: boolean
    busy: boolean
    onImport: () => void
    onAdd: () => void
    onChange: (index: number, field: 'name' | 'url', value: string) => void
    onRemove: (index: number) => void
    onOpenPrefs: (key: string, title: string) => void
}) {
    return (
        <div className="replacements">
            <div className="replacements-head saved-playlists-head">
                <span>Playlist salvate (nome → link)</span>
                <span className="replacements-head-actions">
                    {connected && (
                        <button className="ghost small with-icon" onClick={onImport} disabled={busy}>
                            <span className="btn-icon">
                                <AccountIcon />
                            </span>
                            Importa dall'account
                        </button>
                    )}
                    <button className="ghost small add-replacement" onClick={onAdd} disabled={busy}>
                        + Aggiungi
                    </button>
                </span>
            </div>
            {/* Anche l'ultima riga si toglie con la ✕: senza playlist
                resta solo l'avviso, e "Aggiungi" ne crea una. */}
            {rows.length === 0 && <p className="rule-empty">Nessuna playlist salvata.</p>}
            {rows.map((p, i) => (
                <div className="replacement-row playlist-row" key={i}>
                    <input
                        type="text"
                        placeholder="Nome"
                        value={p.name}
                        onChange={(e) => onChange(i, 'name', e.target.value)}
                        disabled={busy}
                    />
                    <span className="arrow">→</span>
                    <input
                        type="text"
                        placeholder="Link playlist"
                        value={p.url}
                        onChange={(e) => onChange(i, 'url', e.target.value)}
                        disabled={busy}
                    />
                    {/* Impostazioni della playlist: si aprono solo per una
                        riga compilata (nome e link). */}
                    <PrefsButton
                        summary={prefsSummary(prefs[playlistKeyOf(p.url, p.name)])}
                        onClick={() => onOpenPrefs(playlistKeyOf(p.url, p.name), p.name.trim())}
                        disabled={busy || p.name.trim() === '' || p.url.trim() === ''}
                    />
                    <button
                        className="ghost small danger"
                        onClick={() => onRemove(i)}
                        disabled={busy}
                        aria-label="Rimuovi playlist"
                    >
                        ✕
                    </button>
                </div>
            ))}
        </div>
    )
}
