import { type CSSProperties, type ReactNode, type RefObject } from 'react'
import { Collapse, Tooltip } from '../components/controls'
import { LinkField, PlaylistSelect, type PlaylistChoice } from '../components/playlists'
import { AlertIcon, CloseIcon, ConvertIcon, DownloadIcon, PlusIcon, RefreshIcon, TagOffIcon } from '../icons'

// DownloadSource: cosa scaricare, comune a schermata normale e semplificata.
// linkText è il campo del link (onLinkSubmit è il suo Invio); se è compilato
// (hasLink) ha la precedenza e la scelta della playlist si chiude. playlist è
// la scelta fra options; canDownload dice se c'è qualcosa da scaricare.
export type DownloadSource = {
    linkText: string
    onLinkChange: (value: string) => void
    onLinkSubmit: () => void
    onAddToPlaylist?: () => void
    onPaste?: () => void
    hasLink: boolean
    playlist: string
    onPlaylistChange: (value: string) => void
    options: PlaylistChoice[]
    noPlaylists: boolean
    canDownload: boolean
}

// CommandBar: comandi della schermata principale normale, sotto il riepilogo
// delle cartelle (folderLines): link e playlist da scaricare, conversione (o,
// con i risultati, nuova scansione), Annulla e Cancella tag; sotto,
// l'avanzamento (progress). Il link sta su una riga propria, largo quanto la
// riga playlist + "Scarica" (downloadRowWidth, misurata con la playlist
// aperta su downloadRowRef: onDownloadRowSettled la rimisura).
export function CommandBar({
    folderLines,
    source,
    busy,
    foldersOk,
    foldersHint,
    downloadRowWidth,
    downloadRowRef,
    onDownloadRowSettled,
    onDownload,
    downloadErrorCount,
    onShowDownloadErrors,
    hasResults,
    onRefresh,
    onProcess,
    canProcess,
    cancellable,
    onCancel,
    onClearTags,
    canClearTags,
    progress,
}: {
    folderLines: ReactNode
    source: DownloadSource
    busy: boolean
    foldersOk: boolean
    foldersHint: string
    downloadRowWidth: number
    downloadRowRef: RefObject<HTMLDivElement | null>
    onDownloadRowSettled: () => void
    onDownload: () => void
    downloadErrorCount: number
    onShowDownloadErrors: () => void
    hasResults: boolean
    onRefresh: () => void
    onProcess: () => void
    canProcess: boolean
    cancellable: boolean
    onCancel: () => void
    onClearTags: () => void
    canClearTags: boolean
    progress: ReactNode
}) {
    const { noPlaylists } = source
    return (
        <div className="top-row">
            <div className="top-left-head">
                <div className="folder-summary">{folderLines}</div>

                <div className="actions">
                    {/* Link sopra la riga playlist + "Scarica", su una riga
                        propria e largo quanto lei. Il link, se inserito, ha
                        la precedenza: la playlist si chiude (animata) e
                        "Scarica" ne prende il posto. */}
                    <div
                        className="download-box"
                        style={downloadRowWidth ? ({ '--download-row-width': downloadRowWidth + 'px' } as CSSProperties) : undefined}
                    >
                    <LinkField
                        value={source.linkText}
                        onChange={source.onLinkChange}
                        onSubmit={source.onLinkSubmit}
                        onAddToPlaylist={source.onAddToPlaylist}
                        onPaste={source.onPaste}
                        disabled={busy}
                    />
                    <div className="download-controls" ref={downloadRowRef}>
                        <Collapse className="playlist-pick" collapsed={source.hasLink} onSettled={onDownloadRowSettled}>
                            <Tooltip label={noPlaylists ? 'Nessuna playlist: aggiungine una dalle Impostazioni o collega il tuo account Google' : 'Playlist da scaricare'}>
                                <PlaylistSelect
                                    value={source.playlist}
                                    options={source.options}
                                    onChange={source.onPlaylistChange}
                                    disabled={busy || noPlaylists || source.hasLink}
                                />
                            </Tooltip>
                        </Collapse>
                        <Tooltip label={foldersHint || (source.hasLink ? 'Scarica il link inserito' : 'Scarica la playlist selezionata')}>
                            <button
                                className="accent with-icon"
                                onClick={onDownload}
                                disabled={busy || !source.canDownload || !foldersOk}
                            >
                                <span className="btn-icon"><DownloadIcon /></span>
                                Scarica
                            </button>
                        </Tooltip>
                        {downloadErrorCount > 0 && (
                            <Tooltip label={`${downloadErrorCount} download non riusciti: clicca per i dettagli`}>
                                <button
                                    type="button"
                                    className="ghost small with-icon danger download-errors-btn"
                                    onClick={onShowDownloadErrors}
                                    aria-label={`${downloadErrorCount} download non riusciti`}
                                >
                                    <AlertIcon />
                                    {downloadErrorCount}
                                </button>
                            </Tooltip>
                        )}
                    </div>
                    </div>
                    {hasResults ? (
                        <button className="accent with-icon" onClick={onRefresh} disabled={busy || !foldersOk}>
                            <span className="btn-icon"><RefreshIcon /></span>
                            Avvia nuova scansione
                        </button>
                    ) : (
                        <button className="accent with-icon" onClick={onProcess} disabled={!canProcess}>
                            <span className="btn-icon"><ConvertIcon /></span>
                            Converti nomi e scrivi tag
                        </button>
                    )}
                    {busy && cancellable && (
                        <button className="danger-solid with-icon" onClick={onCancel}>
                            <span className="btn-icon"><CloseIcon /></span>
                            Annulla
                        </button>
                    )}
                    <Tooltip label="Cancella tutti i tag ID3 dagli MP3 della cartella">
                        <button
                            className="ghost with-icon danger"
                            onClick={onClearTags}
                            disabled={!canClearTags}
                        >
                            <span className="btn-icon"><TagOffIcon /></span>
                            Cancella tag
                        </button>
                    </Tooltip>
                </div>

                {progress}
            </div>
        </div>
    )
}

// SimpleHero: modalità semplificata, una sola card al centro con la scelta di
// cosa scaricare come protagonista. Niente cartelle (sono nelle Impostazioni:
// qui solo il riepilogo, folderLines), anteprima o registro Attività: gli
// esiti arrivano dai toast e dai risultati. Senza risultati la card è
// centrata anche in verticale; dopo una conversione (hasResults) sale in cima
// e lascia spazio alla tabella.
export function SimpleHero({
    folderLines,
    source,
    busy,
    hasFolder,
    foldersOk,
    foldersHint,
    hasResults,
    onDownload,
    cancellable,
    onCancel,
    onAddPlaylist,
    downloadErrorCount,
    onShowDownloadErrors,
    progress,
}: {
    folderLines: ReactNode
    source: DownloadSource
    busy: boolean
    hasFolder: boolean
    foldersOk: boolean
    foldersHint: string
    hasResults: boolean
    onDownload: () => void
    cancellable: boolean
    onCancel: () => void
    onAddPlaylist: () => void
    downloadErrorCount: number
    onShowDownloadErrors: () => void
    progress: ReactNode
}) {
    const { noPlaylists } = source
    return (
        <div className={'simple-stage' + (hasResults ? ' has-results' : '')}>
            <section className="simple-hero fade-in">
                <div className="simple-hero-badge" aria-hidden="true">
                    <DownloadIcon />
                </div>
                <h2 className="simple-hero-title">Scarica la tua musica</h2>
                <p className="simple-hero-sub">
                    {noPlaylists
                        ? 'Incolla il link di un video, oppure aggiungi una playlist nelle Impostazioni.'
                        : 'Incolla il link di un video o scegli una playlist: i brani vengono scaricati e subito rinominati, con titolo e artista scritti nei tag.'}
                </p>

                {/* Link sopra playlist e azione, largo quanto loro: un
                    blocco unico, largo al massimo 526 px. Il link, se
                    inserito, ha la precedenza sulla playlist. */}
                <div className="simple-hero-download">
                    <LinkField
                        value={source.linkText}
                        onChange={source.onLinkChange}
                        onSubmit={source.onLinkSubmit}
                        onAddToPlaylist={source.onAddToPlaylist}
                        onPaste={source.onPaste}
                        disabled={busy}
                    />

                    <div className="simple-hero-controls">
                        {/* Select + "+" restano affiancati anche quando, su
                            schermi stretti, i comandi si impilano. Il "+"
                            compare solo finché non c'è nessuna playlist. Con un link
                            inserito si chiudono (animati) e resta solo l'azione. */}
                        <Collapse className="simple-hero-pick" collapsed={source.hasLink}>
                            <PlaylistSelect
                                value={source.playlist}
                                options={source.options}
                                onChange={source.onPlaylistChange}
                                disabled={busy || source.hasLink}
                            />
                            {noPlaylists && (
                            <Tooltip label="Aggiungi una playlist nelle Impostazioni">
                                <button
                                    className="ghost simple-hero-add"
                                    onClick={onAddPlaylist}
                                    disabled={busy || source.hasLink}
                                    aria-label="Aggiungi una playlist nelle Impostazioni"
                                >
                                    <PlusIcon />
                                </button>
                            </Tooltip>
                            )}
                        </Collapse>
                        {/* Durante l'operazione il pulsante principale
                            diventa "Annulla": una sola azione alla volta. */}
                        {busy && cancellable ? (
                            <button className="danger-solid simple-hero-action" onClick={onCancel}>
                                <CloseIcon />
                                Annulla
                            </button>
                        ) : (
                            <Tooltip label={!hasFolder ? 'Scegli prima la cartella di partenza nelle Impostazioni' : foldersHint}>
                                <button
                                    className="accent simple-hero-action"
                                    onClick={onDownload}
                                    disabled={busy || !source.canDownload || !foldersOk}
                                >
                                    <DownloadIcon />
                                    Scarica e converti
                                </button>
                            </Tooltip>
                        )}
                    </div>
                </div>

                {progress}

                {downloadErrorCount > 0 && (
                    <button
                        type="button"
                        className="ghost small danger simple-hero-errors"
                        onClick={onShowDownloadErrors}
                    >
                        <AlertIcon />
                        {downloadErrorCount === 1
                            ? '1 download non riuscito'
                            : `${downloadErrorCount} download non riusciti`}
                    </button>
                )}

                {/* Dove finiscono i brani, con la scorciatoia per cambiarlo. */}
                <div className="simple-hero-foot">
                    {folderLines}
                </div>
            </section>
        </div>
    )
}
