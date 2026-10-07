import { AlertIcon, CheckIcon, CloseIcon } from '../icons'

// Toast: notifica effimera (in basso a destra su desktop, in basso a tutta
// larghezza su Android). È l'unico canale per l'esito delle azioni: `ok`
// decide colore/icona, `duration` (ms) la durata prima della chiusura automatica.
export type Toast = { id: number; ok: boolean; message: string; duration: number }

// Toasts: pila dei toast visibili. Ognuno si chiude con la ✕ o da solo alla
// fine dell'animazione della sua barra del tempo (in pausa in hover su desktop).
export function Toasts({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: number) => void }) {
    return (
        <div
            className="toast-container"
            aria-live="polite"
            aria-atomic="false"
        >
            {toasts.map((t) => (
                <div key={t.id} className={'toast ' + (t.ok ? 'toast-ok' : 'toast-err')} role="status">
                    <span className="toast-icon" aria-hidden="true">
                        {t.ok ? <CheckIcon /> : <AlertIcon />}
                    </span>
                    <span className="toast-msg">{t.message}</span>
                    <button
                        type="button"
                        className="toast-close"
                        aria-label="Chiudi notifica"
                        onClick={() => onDismiss(t.id)}
                    >
                        <CloseIcon />
                    </button>
                    {/* Barra del tempo residuo: si svuota in `duration` ms e
                        alla fine della sua animazione chiude il toast. */}
                    <span
                        className="toast-timer"
                        aria-hidden="true"
                        style={{ animationDuration: t.duration + 'ms' }}
                        onAnimationEnd={() => onDismiss(t.id)}
                    />
                </div>
            ))}
        </div>
    )
}
