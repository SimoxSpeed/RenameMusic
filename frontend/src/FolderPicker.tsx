import { useEffect, useRef, useState } from 'react'
import { App as CapApp } from '@capacitor/app'
import { ListDirectory, MakeDirectory, type FolderListing } from './api'

// FolderPicker: selettore di cartelle interno all'app, usato su Android al
// posto del dialog di sistema. Il selettore di Android (SAF) non restituisce
// percorsi reali e da Android 11 non permette di scegliere né la radice della
// memoria né Download; con l'accesso a tutti i file navighiamo direttamente il
// filesystem tramite il core (ListDirectory/MakeDirectory). Il tasto Indietro
// del telefono risale di una cartella, e alla radice chiude il selettore.

function FolderIcon() {
    return (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z" />
        </svg>
    )
}

function UpIcon() {
    return (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M12 19V5" />
            <path d="M5 12l7-7 7 7" />
        </svg>
    )
}

function ChevronIcon() {
    return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M9 6l6 6-6 6" />
        </svg>
    )
}

// displayPath mostra il percorso relativo al volume ("Memoria interna / Music")
// invece del percorso tecnico (/storage/emulated/0/Music).
function displayPath(listing: FolderListing): string {
    const root = [...listing.roots]
        .sort((a, b) => b.path.length - a.path.length)
        .find((r) => listing.path === r.path || listing.path.startsWith(r.path + '/'))
    if (!root) return listing.path
    const rest = listing.path.slice(root.path.length).split('/').filter(Boolean)
    return [root.name, ...rest].join(' / ')
}

export default function FolderPicker({
    title,
    initialPath,
    onCancel,
    onSelect,
}: {
    title: string
    initialPath: string
    onCancel: () => void
    onSelect: (path: string) => void
}) {
    const [listing, setListing] = useState<FolderListing | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState('')
    const [newName, setNewName] = useState<string | null>(null)

    function load(request: Promise<FolderListing>) {
        setLoading(true)
        request
            .then((l) => {
                setListing(l)
                setError(l.error ?? '')
            })
            .catch((e) => setError(String(e?.message ?? e)))
            .finally(() => setLoading(false))
    }

    function open(path: string) {
        setNewName(null)
        load(ListDirectory(path))
    }

    useEffect(() => {
        open(initialPath)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Indietro del telefono: risale di cartella; alla radice chiude.
    const backRef = useRef<() => void>(() => {})
    backRef.current = () => {
        if (newName !== null) setNewName(null)
        else if (listing?.parent) open(listing.parent)
        else onCancel()
    }
    useEffect(() => {
        const handle = CapApp.addListener('backButton', () => backRef.current())
        return () => {
            handle.then((h) => h.remove())
        }
    }, [])

    function createFolder() {
        if (!listing || newName === null) return
        const name = newName.trim()
        if (!name) return
        setNewName(null)
        load(MakeDirectory(listing.path, name))
    }

    const roots = listing?.roots ?? []

    return (
        <div className="modal-overlay">
            <div className="modal folder-picker" onClick={(e) => e.stopPropagation()}>
                <h3>{title}</h3>

                {roots.length > 1 && (
                    <div className="fp-roots">
                        {roots.map((r) => (
                            <button
                                key={r.path}
                                type="button"
                                className={
                                    'fp-root' +
                                    (listing && (listing.path === r.path || listing.path.startsWith(r.path + '/'))
                                        ? ' is-active'
                                        : '')
                                }
                                onClick={() => open(r.path)}
                            >
                                {r.name}
                            </button>
                        ))}
                    </div>
                )}

                <div className="fp-path">{listing ? displayPath(listing) : '…'}</div>

                <ul className={'fp-list' + (loading ? ' is-loading' : '')}>
                    {listing?.parent && (
                        <li>
                            <button type="button" className="fp-item fp-up" onClick={() => open(listing.parent)}>
                                <UpIcon />
                                <span className="fp-name">Cartella superiore</span>
                            </button>
                        </li>
                    )}
                    {listing?.dirs.map((d) => (
                        <li key={d}>
                            <button type="button" className="fp-item" onClick={() => open(listing.path + '/' + d)}>
                                <FolderIcon />
                                <span className="fp-name">{d}</span>
                                <ChevronIcon />
                            </button>
                        </li>
                    ))}
                    {listing && listing.dirs.length === 0 && !error && (
                        <li className="fp-empty">Nessuna sottocartella.</li>
                    )}
                </ul>

                {error && <p className="fp-error">{error}</p>}

                {newName !== null ? (
                    <div className="fp-new">
                        <input
                            type="text"
                            placeholder="Nome della nuova cartella"
                            value={newName}
                            onChange={(e) => setNewName(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter') createFolder()
                            }}
                            // eslint-disable-next-line jsx-a11y/no-autofocus
                            autoFocus
                        />
                        <button type="button" onClick={() => setNewName(null)}>
                            Annulla
                        </button>
                        <button type="button" className="primary" onClick={createFolder} disabled={!newName.trim()}>
                            Crea
                        </button>
                    </div>
                ) : (
                    <button
                        type="button"
                        className="ghost small fp-new-btn"
                        onClick={() => setNewName('')}
                        disabled={!listing || loading}
                    >
                        + Nuova cartella
                    </button>
                )}

                <div className="modal-actions">
                    <button type="button" onClick={onCancel}>
                        Annulla
                    </button>
                    <button
                        type="button"
                        className="accent"
                        onClick={() => listing && onSelect(listing.path)}
                        disabled={!listing || loading}
                    >
                        Usa questa cartella
                    </button>
                </div>
            </div>
        </div>
    )
}
