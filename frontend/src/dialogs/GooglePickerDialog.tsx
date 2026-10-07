import { type Dispatch, type SetStateAction } from 'react'
import { type core } from '../api'
import { Modal } from '../components/Modal'
import { PlusIcon } from '../icons'

// GooglePickerDialog: playlist dell'account Google, da spuntare per importarle
// nelle Impostazioni (mode 'import': selected sono gli ID spuntati, savedIds
// quelli già salvati, che non si possono riaggiungere) oppure da scegliere per
// aggiungerci il video del link (mode 'add'). lists è null finché l'elenco
// non arriva.
export function GooglePickerDialog({
    mode,
    lists,
    savedIds,
    selected,
    setSelected,
    onClose,
    onImport,
    onAdd,
}: {
    mode: 'import' | 'add'
    lists: core.GooglePlaylistView[] | null
    savedIds: Set<string>
    selected: Set<string>
    setSelected: Dispatch<SetStateAction<Set<string>>>
    onClose: () => void
    onImport: () => void
    onAdd: (p: core.GooglePlaylistView) => void
}) {
    return (
        <Modal onClose={onClose} wide>
            <h3>{mode === 'import' ? 'Importa le tue playlist' : 'Aggiungi a una playlist'}</h3>
            <p>
                {mode === 'import'
                    ? 'Scegli le playlist del tuo account YouTube da aggiungere a quelle da scaricare.'
                    : 'Scegli la playlist del tuo account in cui aggiungere il video del link.'}
            </p>
            {lists === null ? (
                <p className="google-hint" role="status">
                    <span className="spinner" aria-hidden="true" />
                    Lettura delle playlist…
                </p>
            ) : lists.length === 0 ? (
                <p className="google-hint">Il tuo account non ha ancora playlist.</p>
            ) : (
                <ul className="google-list">
                    {lists.map((p) => {
                        const saved = savedIds.has(p.id)
                        const meta = [
                            p.count === 1 ? '1 video' : `${p.count} video`,
                            p.privacy === 'private'
                                ? 'privata'
                                : p.privacy === 'unlisted'
                                  ? 'non in elenco'
                                  : 'pubblica',
                        ].join(' · ')
                        return mode === 'import' ? (
                            <li key={p.id}>
                                <label className={'google-item' + (saved ? ' is-disabled' : '')}>
                                    <input
                                        type="checkbox"
                                        checked={saved || selected.has(p.id)}
                                        disabled={saved}
                                        onChange={(e) =>
                                            setSelected((prev) => {
                                                const next = new Set(prev)
                                                if (e.target.checked) next.add(p.id)
                                                else next.delete(p.id)
                                                return next
                                            })
                                        }
                                    />
                                    <span className="google-item-text">
                                        <span className="google-item-title">{p.title}</span>
                                        <span className="google-item-meta">{saved ? 'Già salvata' : meta}</span>
                                    </span>
                                </label>
                            </li>
                        ) : (
                            <li key={p.id}>
                                <button type="button" className="google-item" onClick={() => onAdd(p)}>
                                    <span className="google-item-text">
                                        <span className="google-item-title">{p.title}</span>
                                        <span className="google-item-meta">{meta}</span>
                                    </span>
                                    <PlusIcon />
                                </button>
                            </li>
                        )
                    })}
                </ul>
            )}
            <div className="modal-actions">
                <button onClick={onClose}>Annulla</button>
                {mode === 'import' && (
                    <button className="primary" onClick={onImport} disabled={selected.size === 0}>
                        {selected.size > 0 ? `Importa (${selected.size})` : 'Importa'}
                    </button>
                )}
            </div>
        </Modal>
    )
}
