import { type core } from '../api'
import { Tooltip } from '../components/controls'
import { CurrentField, ErrorLabel, ExtChip } from '../components/files'
import { ConvertIcon, EyeIcon, RefreshIcon } from '../icons'
import { splitName, tagChanged } from '../lib/files'

// ResultsTable: esito dell'ultima conversione, una riga per file.
function ResultsTable({ results }: { results: core.ResultView[] }) {
    return (
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
                            <td data-label="Nome originale">
                                {renamed ? <s className="old-name">{src.base}</s> : src.base}
                            </td>
                            <td data-label="Nuovo nome">{r.skipped || r.canceled ? '—' : dst.base}</td>
                            <td data-label="Titolo">{showTags ? r.title : <span className="muted-dash">—</span>}</td>
                            <td data-label="Artista">{showTags ? r.artist : <span className="muted-dash">—</span>}</td>
                            <td data-label="Esito">
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
                                        {r.tagged && <span className="badge badge-tag">Taggato</span>}
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
}

// PreviewTable: anteprima dei file (files, già filtrati dalla vista "Solo da
// modificare"), con la casella per segnare ogni file da rivedere (reviewPaths)
// e quella dell'intestazione per tutti quelli visibili.
function PreviewTable({
    files,
    reviewPaths,
    busy,
    onToggleReview,
    onToggleReviewAll,
}: {
    files: core.FileView[]
    reviewPaths: Set<string>
    busy: boolean
    onToggleReview: (path: string, checked: boolean) => void
    onToggleReviewAll: (checked: boolean) => void
}) {
    const allVisibleSelected = files.length > 0 && files.every((f) => reviewPaths.has(f.path))
    const someVisibleSelected = files.some((f) => reviewPaths.has(f.path))
    return (
        <table className="preview-table">
            <thead>
                <tr>
                    <th className="cell-select">
                        <input
                            type="checkbox"
                            aria-label="Seleziona tutti da rivedere"
                            title="Seleziona tutti da rivedere"
                            checked={allVisibleSelected}
                            ref={(el) => {
                                if (el) el.indeterminate = someVisibleSelected && !allVisibleSelected
                            }}
                            onChange={(e) => onToggleReviewAll(e.target.checked)}
                            disabled={busy}
                        />
                    </th>
                    <th className="cell-current">File attuale</th>
                    <th>Anteprima nuovo nome</th>
                    <th>Anteprima nuovo titolo</th>
                    <th>Anteprima nuovo artista</th>
                    <th>Stato</th>
                </tr>
            </thead>
            <tbody>
                {files.map((file, i) => {
                    const src = splitName(file.name)
                    const dst = splitName(file.preview)
                    const nameChanged = src.base !== dst.base
                    const titleChanged = file.mp3 && tagChanged(file.title, file.titlePreview)
                    const artistChanged = file.mp3 && tagChanged(file.artist, file.artistPreview)
                    const rowChanged = nameChanged || titleChanged || artistChanged
                    const toReview = reviewPaths.has(file.path)
                    return (
                        <tr key={i} className={(rowChanged ? 'changed' : '') + (toReview ? ' to-review' : '')}>
                            <td className="cell-select">
                                <input
                                    type="checkbox"
                                    aria-label={'Rivedi ' + src.base}
                                    checked={toReview}
                                    onChange={(e) => onToggleReview(file.path, e.target.checked)}
                                    disabled={busy}
                                />
                            </td>
                            <td data-label="File attuale" className="cell-current">
                                <CurrentField label="nome" value={src.base} changed={nameChanged} />
                                {file.mp3 && (
                                    <CurrentField label="titolo" value={file.title ?? ''} changed={titleChanged} />
                                )}
                                {file.mp3 && (
                                    <CurrentField label="artista" value={file.artist ?? ''} changed={artistChanged} />
                                )}
                            </td>
                            <td data-label="Nuovo nome" className={nameChanged ? 'value-changed' : ''}>
                                {dst.base}
                            </td>
                            <td data-label="Nuovo titolo" className={titleChanged ? 'value-changed' : ''}>
                                {file.mp3 ? file.titlePreview : <span className="muted-dash">—</span>}
                            </td>
                            <td data-label="Nuovo artista" className={artistChanged ? 'value-changed' : ''}>
                                {file.mp3 ? file.artistPreview : <span className="muted-dash">—</span>}
                            </td>
                            <td data-label="Stato">
                                <div className="badges">
                                    {nameChanged ? (
                                        <span className="badge badge-changed">Da rinominare</span>
                                    ) : (
                                        <span className="badge badge-neutral">Invariato</span>
                                    )}
                                    {(titleChanged || artistChanged) && (
                                        <span className="badge badge-tag">Da taggare</span>
                                    )}
                                    {toReview && <span className="badge badge-review">Da rivedere</span>}
                                    <ExtChip ext={src.ext} />
                                </div>
                            </td>
                        </tr>
                    )
                })}
            </tbody>
        </table>
    )
}

// PreviewPanel: anteprima della cartella o, dopo una conversione, i suoi
// risultati (results). In modalità semplificata (simple) compare solo con i
// risultati. previewFiles sono i file mostrati (con "Solo da modificare",
// onlyChanged, solo quelli che cambiano o sono da rivedere); files tutti
// quelli della cartella, per distinguere i messaggi di vuoto.
export function PreviewPanel({
    results,
    simple,
    busy,
    booted,
    folder,
    folderMissing,
    foldersOk,
    foldersHint,
    files,
    previewFiles,
    onlyChanged,
    onOnlyChangedChange,
    onRefresh,
    reviewPaths,
    onToggleReview,
    onToggleReviewAll,
}: {
    results: core.ResultView[] | null
    simple: boolean
    busy: boolean
    booted: boolean
    folder: string
    folderMissing: boolean
    foldersOk: boolean
    foldersHint: string
    files: core.FileView[]
    previewFiles: core.FileView[]
    onlyChanged: boolean
    onOnlyChangedChange: (checked: boolean) => void
    onRefresh: () => void
    reviewPaths: Set<string>
    onToggleReview: (path: string, checked: boolean) => void
    onToggleReviewAll: (checked: boolean) => void
}) {
    return (
        <section className={'panel preview-panel fade-in' + (simple ? ' simple-results' : '')}>
            <div className="panel-head">
                <h2>
                    <span className="h2-icon">{results ? <ConvertIcon /> : <EyeIcon />}</span>
                    {results ? 'Risultato conversione' : 'Anteprima'}
                </h2>
                {!results && (
                    <div className="preview-tools">
                        <Tooltip label="Mostra solo i file che subiranno una modifica, nel nome o nei tag, e quelli selezionati da rivedere. È solo una vista: l'elaborazione tratta comunque tutti i file.">
                            <label className="toggle-changed">
                                <input
                                    type="checkbox"
                                    checked={onlyChanged}
                                    onChange={(e) => onOnlyChangedChange(e.target.checked)}
                                    disabled={busy}
                                />
                                Solo da modificare
                            </label>
                        </Tooltip>
                        <Tooltip label={foldersHint || 'Aggiorna la scansione della cartella'}>
                            <button className="ghost small with-icon" onClick={onRefresh} disabled={busy || !foldersOk}>
                                <span className="btn-icon">
                                    <RefreshIcon />
                                </span>
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
                    <ResultsTable results={results} />
                )
            ) : !booted ? (
                <div className="empty">Caricamento…</div>
            ) : !folder ? (
                <div className="empty">Scegli una cartella di partenza nelle Impostazioni per vedere l'anteprima.</div>
            ) : folderMissing ? (
                <div className="empty">Cartella di partenza non trovata.</div>
            ) : files.length === 0 ? (
                <div className="empty">Nessun file MP3 nella cartella di partenza.</div>
            ) : previewFiles.length === 0 ? (
                <div className="empty">Nessun file da modificare.</div>
            ) : (
                <PreviewTable
                    files={previewFiles}
                    reviewPaths={reviewPaths}
                    busy={busy}
                    onToggleReview={onToggleReview}
                    onToggleReviewAll={onToggleReviewAll}
                />
            )}
        </section>
    )
}
