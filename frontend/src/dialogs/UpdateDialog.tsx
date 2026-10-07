import { isAndroid, type core } from '../api'
import { OpProgress } from '../components/controls'
import { Modal } from '../components/Modal'
import { formatMB, installLabel, installPercent, type InstallProgress } from '../lib/progress'

// UpdateDialog: nuova versione dell'app, con le note della release e, durante
// l'installazione, l'avanzamento del download. Mentre l'installazione è in
// corso (busy) non si chiude.
export function UpdateDialog({
    update,
    appVersion,
    busy,
    installProgress,
    onClose,
    onInstall,
}: {
    update: core.UpdateView
    appVersion: string
    busy: boolean
    installProgress: InstallProgress | null
    onClose: () => void
    onInstall: () => void
}) {
    return (
        <Modal onClose={() => !busy && onClose()}>
            <h3>Nuova versione disponibile</h3>
            <p>
                È disponibile <strong>RenameMusic {update.version}</strong> (stai usando la{' '}
                {appVersion}).{' '}
                {isAndroid
                    ? "L'app scaricherà l'aggiornamento e aprirà l'installazione di Android."
                    : "L'app scaricherà la nuova versione e si riavvierà da sola."}
                {update.size > 0 && <> Download: {formatMB(update.size)} MB.</>}
            </p>
            {update.notes && <div className="update-notes">{update.notes}</div>}
            {busy && installProgress?.tool === 'RenameMusic' && (
                <OpProgress
                    className="update-progress"
                    percent={installPercent(installProgress)}
                    label={installLabel(installProgress)}
                />
            )}
            <p className="update-later">Puoi aggiornare anche più tardi dalla scheda Info delle Impostazioni.</p>
            <div className="modal-actions">
                <button onClick={onClose} disabled={busy}>
                    Più tardi
                </button>
                <button className="accent" onClick={onInstall} disabled={busy}>
                    Aggiorna ora
                </button>
            </div>
        </Modal>
    )
}
