import { type ReactNode } from 'react'
import { AlertIcon, FolderInIcon, FolderInOutIcon, FolderOutIcon } from '../icons'
import { Tooltip } from './controls'

// MissingFolderIcon: icona d'errore accanto a una cartella impostata ma non
// trovata sul disco; il dettaglio compare solo in hover. Con onClick (fuori
// dalle Impostazioni) è un pulsante che porta dove la si sceglie di nuovo.
export function MissingFolderIcon({ label, onClick, disabled }: { label: string; onClick?: () => void; disabled?: boolean }) {
    const detail = label + ' non trovata: è stata spostata, rinominata o eliminata.'
    if (!onClick) {
        return (
            <Tooltip label={detail}>
                <span className="folder-missing" role="img" aria-label={label + ' non trovata'}>
                    <AlertIcon />
                </span>
            </Tooltip>
        )
    }
    return (
        <Tooltip label={detail + ' Clicca per sceglierla di nuovo nelle Impostazioni.'}>
            <button
                type="button"
                className="folder-missing"
                onClick={onClick}
                disabled={disabled}
                aria-label={label + ' non trovata: apri le Impostazioni'}
            >
                <AlertIcon />
            </button>
        </Tooltip>
    )
}

// FolderLine: una riga del riepilogo, con l'icona che la distingue e il
// percorso completo (oppure un testo se la cartella non è impostata).
export function FolderLine(props: {
    icon: ReactNode
    kind: string
    path: string
    placeholder: string
    missing: boolean
    onMissingClick: () => void
    disabled: boolean
}) {
    const { icon, kind, path, placeholder, missing, onMissingClick, disabled } = props
    return (
        <div className="folder-line">
            <span className="folder-line-icon" role="img" aria-label={kind}>
                {icon}
            </span>
            <span className="folder-line-path">{path || <em>{placeholder}</em>}</span>
            {missing && <MissingFolderIcon label={kind} onClick={onMissingClick} disabled={disabled} />}
        </div>
    )
}

// FolderLines riepiloga le cartelle su due righe: partenza (dove si
// leggono/scaricano i brani) e destinazione (dove finiscono quelli
// convertiti); se coincidono, una riga sola con l'icona combinata. Una
// cartella impostata ma non trovata sul disco è segnalata sulla sua riga
// (onMissingClick: click sull'icona d'errore).
export function FolderLines(props: {
    folder: string
    destSameAsSource: boolean
    destFolder: string
    folderMissing: boolean
    destMissing: boolean
    onMissingClick: () => void
    disabled: boolean
}) {
    const { folder, destSameAsSource, destFolder, folderMissing, destMissing, onMissingClick, disabled } = props
    if (destSameAsSource) {
        return (
            <div className="folder-lines">
                <FolderLine
                    icon={<FolderInOutIcon />}
                    kind="Cartella di partenza e destinazione"
                    path={folder}
                    placeholder="Nessuna cartella di partenza"
                    missing={folderMissing}
                    onMissingClick={onMissingClick}
                    disabled={disabled}
                />
            </div>
        )
    }
    return (
        <div className="folder-lines">
            <FolderLine
                icon={<FolderInIcon />}
                kind="Cartella di partenza"
                path={folder}
                placeholder="Nessuna cartella di partenza"
                missing={folderMissing}
                onMissingClick={onMissingClick}
                disabled={disabled}
            />
            <FolderLine
                icon={<FolderOutIcon />}
                kind="Cartella di destinazione"
                path={destFolder}
                placeholder="Nessuna cartella di destinazione"
                missing={destMissing}
                onMissingClick={onMissingClick}
                disabled={disabled}
            />
        </div>
    )
}
