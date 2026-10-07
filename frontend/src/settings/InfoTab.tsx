import { openURL, type core } from '../api'
import { Tooltip } from '../components/controls'
import { DownloadIcon, RefreshIcon } from '../icons'

// Repository del progetto, linkato nella scheda Info delle Impostazioni.
const REPO_URL = 'https://github.com/SimoxSpeed/RenameMusic'

// InfoTab: scheda Info, con versione e aggiornamenti (update è la nuova
// versione disponibile, se c'è), autore, link al repo e scarico di
// responsabilità.
export function InfoTab({
    appVersion,
    update,
    busy,
    onShowUpdate,
    onCheckUpdate,
}: {
    appVersion: string
    update: core.UpdateView | undefined
    busy: boolean
    onShowUpdate: (update: core.UpdateView) => void
    onCheckUpdate: () => void
}) {
    return (
        <section className="settings about">
            <h2>RenameMusic</h2>
            <p className="about-desc">
                RenameMusic scarica le tue playlist di YouTube direttamente in MP3 e mette in ordine la tua musica in un
                clic, con nomi puliti e tag di titolo e artista coerenti.
            </p>
            <div className="ytdlp-panel">
                <div className="ytdlp-head">
                    <span className="ytdlp-title">Versione {appVersion}</span>
                    {update ? (
                        <>
                            <span className="ytdlp-badge update-badge">Disponibile la versione {update.version}</span>
                            <button
                                className="ghost small with-icon ytdlp-install"
                                onClick={() => onShowUpdate(update)}
                                disabled={busy}
                            >
                                <span className="btn-icon">
                                    <DownloadIcon />
                                </span>
                                Aggiorna
                            </button>
                        </>
                    ) : (
                        <Tooltip label="Controlla subito su GitHub se è uscita una nuova versione (l'app lo fa comunque da sola quando è connessa a Internet)">
                            <button
                                className="ghost small with-icon ytdlp-install"
                                onClick={onCheckUpdate}
                                disabled={busy}
                            >
                                <span className="btn-icon">
                                    <RefreshIcon />
                                </span>
                                Verifica aggiornamenti
                            </button>
                        </Tooltip>
                    )}
                </div>
            </div>

            <dl className="about-list">
                <dt>Autore</dt>
                <dd>Simone D'Alessandro</dd>
                <dt>Codice sorgente</dt>
                <dd>
                    <a
                        href={REPO_URL}
                        onClick={(e) => {
                            e.preventDefault()
                            openURL(REPO_URL)
                        }}
                    >
                        github.com/SimoxSpeed/RenameMusic
                    </a>
                </dd>
            </dl>

            <hr className="settings-divider" />

            <div className="about-disclaimer">
                <h3>Esclusione di responsabilità</h3>
                <p>
                    RenameMusic è distribuito così com'è, senza alcuna garanzia. Rinomina e sposta file e, se lo attivi,
                    elimina gli originali: prima di usarlo su una raccolta a cui tieni, fanne una copia. L'autore non
                    risponde di perdite di dati o di altri danni causati dall'uso dell'app.
                </p>
                <p>
                    Scaricare da YouTube può violare i suoi Termini di servizio e il diritto d'autore: scarica solo
                    contenuti di cui hai i diritti o che sono distribuiti liberamente. L'uso che ne fai è sotto la tua
                    responsabilità.
                </p>
            </div>
        </section>
    )
}
