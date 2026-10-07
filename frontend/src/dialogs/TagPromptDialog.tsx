import { ExtChip } from '../components/files'
import { Modal } from '../components/Modal'
import { SearchIcon } from '../icons'
import { UNKNOWN_ARTIST, UNKNOWN_TITLE, foldText, type PromptSearch, type TagPrompt } from '../lib/prompts'

// TagPromptDialog: popup della traccia in testa alla coda delle tracce da
// confermare (tag sconosciuti o selezionata da rivedere). Con la ricerca su
// MusicBrainz attiva prima mostra solo l'attesa («Salta ricerca»), poi il
// campo col nome (draft), i brani trovati da filtrare e scegliere e il link
// per tornare al nome di prima. Si chiude solo con una scelta: onResolve(true)
// usa il nome del campo, onResolve(false) salta o usa l'anteprima.
export function TagPromptDialog({
    prompts,
    search,
    draft,
    onDraftChange,
    filter,
    onFilterChange,
    onSkipSearch,
    onResolve,
}: {
    prompts: TagPrompt[]
    search: PromptSearch | null
    draft: string
    onDraftChange: (value: string) => void
    filter: string
    onFilterChange: (value: string) => void
    onSkipSearch: () => void
    onResolve: (useEdited: boolean) => void
}) {
    const head = prompts[0]
    const titleUnknown = head.title === UNKNOWN_TITLE
    const artistUnknown = head.artist === UNKNOWN_ARTIST
    const missing = titleUnknown && artistUnknown
        ? 'né il titolo né l’artista'
        : titleUnknown
          ? 'il titolo'
          : 'l’artista'
    const searching = !!search?.loading
    const found = search?.names ?? []
    const filterWords = foldText(filter).split(/\s+/).filter(Boolean)
    const shown = found.filter((name) => {
        const folded = foldText(name)
        return filterWords.every((w) => folded.includes(w))
    })
    const originalDraft = head.review ? head.previewBase : head.originalBase
    const queue = prompts.length > 1 && (
        <p className="tag-prompt-queue">Altre {prompts.length - 1} tracce in attesa di una scelta.</p>
    )
    if (searching) {
        return (
            <Modal>
                <h3>{head.review ? 'Traccia da rivedere' : 'Traccia non rinominabile'}</h3>
                <p className="tag-prompt-searching" role="status">
                    <span className="spinner" aria-hidden="true" />
                    <span>
                        Ricerca di <strong>{head.originalBase}</strong> su MusicBrainz…
                    </span>
                </p>
                {queue}
                <div className="modal-actions">
                    <button onClick={onSkipSearch}>Salta ricerca</button>
                </div>
            </Modal>
        )
    }
    return (
        <Modal>
            {head.review ? (
                <>
                    <h3>Traccia da rivedere</h3>
                    {found.length > 0 ? (
                        <p>
                            Hai scelto di rivedere questa traccia prima della conversione. Il nome qui
                            sotto è quello trovato su MusicBrainz (l'anteprima proponeva titolo{' '}
                            <strong>{head.title}</strong>, artista <strong>{head.artist}</strong>):
                            controllalo e conferma,{' '}
                            {found.length > 1 && 'scegline un altro dall’elenco, '}
                            <strong>correggilo</strong> (i tag verranno riestratti da esso) oppure
                            converti la traccia come in anteprima.
                        </p>
                    ) : (
                        <p>
                            Hai scelto di rivedere questa traccia prima della conversione. Il nome qui
                            sotto è quello proposto dall'anteprima (titolo <strong>{head.title}</strong>,
                            artista <strong>{head.artist}</strong>): puoi{' '}
                            <strong>correggerlo</strong> (i tag verranno riestratti da esso) oppure
                            convertire la traccia come in anteprima.
                        </p>
                    )}
                </>
            ) : (
                <>
                    <h3>Traccia non rinominabile</h3>
                    {found.length > 1 ? (
                        <p>
                            Dal nome di questa traccia non è possibile dedurre <strong>{missing}</strong>.
                            Su MusicBrainz ci sono più brani che corrispondono: nel campo c'è il più
                            diffuso, puoi sceglierne un altro dall'elenco, <strong>correggere il nome</strong>{' '}
                            (i tag verranno riestratti da esso) oppure saltare per lasciarlo invariato.
                        </p>
                    ) : found.length === 1 ? (
                        <p>
                            Dal nome di questa traccia non è possibile dedurre <strong>{missing}</strong>,
                            ma su MusicBrainz è stata trovata la corrispondenza qui sotto: controllala e
                            conferma, <strong>correggila</strong> (i tag verranno riestratti dal nome)
                            oppure salta per lasciare il nome invariato.
                        </p>
                    ) : (
                        <p>
                            Dal nome di questa traccia non è possibile dedurre <strong>{missing}</strong>:
                            così com'è non può essere rinominata né taggata correttamente. Puoi{' '}
                            <strong>correggere il nome</strong> qui sotto (i tag verranno riestratti da esso)
                            oppure procedere lasciandolo invariato.
                        </p>
                    )}
                </>
            )}
            <label className="tag-prompt-field">
                {/* Etichetta = nome originale della traccia: resta visibile
                    mentre lo si modifica nel campo. */}
                <span className="tag-prompt-label" title="Nome originale">{head.originalBase}</span>
                <div className="tag-prompt-input">
                    <input
                        type="text"
                        value={draft}
                        onChange={(e) => onDraftChange(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') onResolve(true)
                        }}
                        // eslint-disable-next-line jsx-a11y/no-autofocus
                        autoFocus
                    />
                    <ExtChip ext={head.ext} />
                </div>
            </label>
            {search && !search.skipped && (
                <p className={'tag-prompt-suggest' + (search.error ? ' is-error' : '')}>
                    {search.error ? (
                        <>Ricerca non riuscita: {search.error}.</>
                    ) : found.length === 0 ? (
                        <>Nessun risultato su MusicBrainz.</>
                    ) : (
                        <>
                            {found.length > 1 ? 'Proposti da MusicBrainz' : 'Proposto da MusicBrainz'} ·{' '}
                            <button
                                type="button"
                                className="tag-prompt-restore"
                                onClick={() => onDraftChange(originalDraft)}
                                disabled={draft === originalDraft}
                            >
                                {head.review ? 'Usa il nome dell’anteprima' : 'Usa il nome originale'}
                            </button>
                        </>
                    )}
                </p>
            )}
            {found.length > 1 && (
                <div className="tag-prompt-results">
                    <div className="tag-prompt-filter">
                        <SearchIcon />
                        <input
                            type="search"
                            value={filter}
                            onChange={(e) => onFilterChange(e.target.value)}
                            onKeyDown={(e) => {
                                // Invio sceglie il primo brano filtrato; Esc svuota la ricerca
                                // senza chiudere nulla.
                                if (e.key === 'Enter' && shown.length > 0) onDraftChange(shown[0])
                                if (e.key === 'Escape' && filter) {
                                    e.stopPropagation()
                                    onFilterChange('')
                                }
                            }}
                            placeholder={`Cerca tra i ${found.length} risultati`}
                            aria-label="Cerca tra i risultati di MusicBrainz"
                        />
                        <span className="tag-prompt-count">
                            {shown.length === found.length ? found.length : `${shown.length} di ${found.length}`}
                        </span>
                    </div>
                    {shown.length > 0 ? (
                        <ul className="tag-prompt-options">
                            {shown.map((name) => (
                                <li key={name}>
                                    <button
                                        type="button"
                                        className={'tag-prompt-option' + (name === draft ? ' is-selected' : '')}
                                        aria-pressed={name === draft}
                                        onClick={() => onDraftChange(name)}
                                    >
                                        {name}
                                    </button>
                                </li>
                            ))}
                        </ul>
                    ) : (
                        <p className="tag-prompt-empty">Nessun brano corrisponde a «{filter.trim()}».</p>
                    )}
                </div>
            )}
            {queue}
            <div className="modal-actions">
                <button onClick={() => onResolve(false)}>
                    {head.review ? 'Usa anteprima' : 'Salta'}
                </button>
                <button className="accent" onClick={() => onResolve(true)}>
                    Continua
                </button>
            </div>
        </Modal>
    )
}
