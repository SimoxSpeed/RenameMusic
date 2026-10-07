import { type ReactNode } from 'react'

// Modal: popup centrato sopra un velo scuro. onClose, se c'è, lo chiude con un
// clic sul velo (fuori dal popup); wide lo allarga per gli elenchi.
export function Modal({ onClose, wide, children }: { onClose?: () => void; wide?: boolean; children: ReactNode }) {
    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className={'modal' + (wide ? ' modal-wide' : '')} onClick={(e) => e.stopPropagation()}>
                {children}
            </div>
        </div>
    )
}

// ConfirmDialog: popup di conferma con titolo, testo (children) e i tasti
// Annulla e di conferma; il velo e Annulla chiudono senza confermare.
// confirmClassName è lo stile del tasto di conferma (es. danger-solid).
export function ConfirmDialog({
    title,
    confirmLabel,
    confirmClassName,
    onCancel,
    onConfirm,
    disabled,
    children,
}: {
    title: ReactNode
    confirmLabel: ReactNode
    confirmClassName: string
    onCancel: () => void
    onConfirm: () => void
    disabled?: boolean
    children: ReactNode
}) {
    return (
        <Modal onClose={onCancel}>
            <h3>{title}</h3>
            {children}
            <div className="modal-actions">
                <button onClick={onCancel} disabled={disabled}>
                    Annulla
                </button>
                <button className={confirmClassName} onClick={onConfirm} disabled={disabled}>
                    {confirmLabel}
                </button>
            </div>
        </Modal>
    )
}
