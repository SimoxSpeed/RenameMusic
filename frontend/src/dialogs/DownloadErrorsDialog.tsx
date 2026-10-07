import { type core } from '../api'
import { Modal } from '../components/Modal'

// DownloadErrorsDialog: video non scaricati nell'ultimo download, con il
// dettaglio dell'errore di ciascuno.
export function DownloadErrorsDialog({ errors, onClose }: { errors: core.DownloadErrorView[]; onClose: () => void }) {
    return (
        <Modal onClose={onClose} wide>
            <h3>Download non riusciti</h3>
            <p>
                Questi {errors.length} video non sono stati scaricati. Puoi riprovare più tardi: i file già scaricati
                non vengono riscaricati.
            </p>
            <ul className="download-errors-list">
                {errors.map((e, i) => (
                    <li key={i} className="download-error-item">
                        <div className="dl-err-title">{e.title || e.videoId}</div>
                        {e.url && <div className="dl-err-url">{e.url}</div>}
                        <div className="dl-err-msg">{e.message || 'Errore sconosciuto.'}</div>
                    </li>
                ))}
            </ul>
            <div className="modal-actions">
                <button className="primary" onClick={onClose}>
                    Chiudi
                </button>
            </div>
        </Modal>
    )
}
