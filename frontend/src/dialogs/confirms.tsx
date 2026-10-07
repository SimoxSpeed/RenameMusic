import { isAndroid } from '../api'
import { ConfirmDialog, Modal } from '../components/Modal'

// Popup di conferma delle azioni distruttive o lunghe: ognuno riceve
// onCancel/onConfirm e disabled (operazione in corso), il resto è il testo.

export function EmptyPlaylistConfirm({ title, onCancel, onConfirm, disabled }: {
    title: string
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title={<>Svuotare «{title}»?</>}
            confirmLabel="Svuota"
            confirmClassName="danger-solid"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                Verranno <strong>tolti da YouTube tutti i brani</strong> della playlist, anche
                quelli che non hai ancora scaricato. La playlist resta, vuota. L'operazione
                non si può annullare a cose fatte.
            </p>
        </ConfirmDialog>
    )
}

export function GoogleSignOutConfirm({ onCancel, onConfirm, disabled }: {
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Scollegare l'account Google?"
            confirmLabel="Scollega"
            confirmClassName="danger-solid"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                L'app perderà l'accesso alle tue playlist di YouTube e il permesso verrà
                revocato anche su Google. Le playlist del tuo account spariscono dalla
                scelta del download; quelle salvate qui restano, ma finché non ricolleghi
                l'account dopo il download non verranno svuotate né copiate.
            </p>
            <p>
                Regole, playlist e predefiniti restano quelli attuali su questo dispositivo,
                ma non si sincronizzano più con gli altri.
            </p>
        </ConfirmDialog>
    )
}

export function DeleteOriginalsConfirm({ onCancel, onConfirm, disabled }: {
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Attivare l'eliminazione degli originali?"
            confirmLabel="Continua"
            confirmClassName="danger-solid"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                Con questa opzione attiva, dopo ogni conversione i file originali
                verranno <strong>eliminati definitivamente</strong>. Verifica di avere
                un backup se ti serve poter tornare indietro.
            </p>
        </ConfirmDialog>
    )
}

export function ClearTagsConfirm({ onCancel, onConfirm, disabled }: {
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Cancellare tutti i tag?"
            confirmLabel="Cancella tag"
            confirmClassName="danger-solid"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                Verranno <strong>rimossi tutti i tag ID3</strong> (titolo, artista, ecc.)
                da tutti gli MP3 della cartella di partenza. I file non vengono rinominati
                né spostati, ma i metadati eliminati <strong>non sono recuperabili</strong>.
            </p>
        </ConfirmDialog>
    )
}

// InstallYtDlpConfirm: "Scarica" senza yt-dlp. Il testo dipende da come
// l'app gestisce yt-dlp (integrato su Android, copia gestita o percorso
// personalizzato); downloadWhat è "del link" o "della playlist".
export function InstallYtDlpConfirm({ managed, ffmpegAvailable, downloadWhat, onCancel, onConfirm, disabled }: {
    managed: boolean
    ffmpegAvailable: boolean
    downloadWhat: string
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title={
                isAndroid
                    ? 'yt-dlp non è ancora pronto'
                    : managed
                      ? 'Scaricare yt-dlp?'
                      : 'Attivare la gestione automatica di yt-dlp?'
            }
            confirmLabel={managed || isAndroid ? 'Scarica e continua' : 'Attiva e continua'}
            confirmClassName="accent"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            {isAndroid ? (
                <p>
                    L'app lo sta ancora preparando (al primo avvio richiede qualche
                    secondo) oppure la preparazione non è riuscita. Vuoi scaricarne
                    l'ultima versione e avviare subito il download {downloadWhat}?
                </p>
            ) : managed ? (
                <p>
                    yt-dlp non è presente. L'app lo scaricherà
                    {!ffmpegAvailable && <> insieme a ffmpeg (circa 200 MB)</>} in{' '}
                    <code>%AppData%\RenameMusic</code> e avvierà subito il download
                    {downloadWhat}.
                </p>
            ) : (
                <p>
                    <strong>"Gestisci autonomamente"</strong> non è attivo e yt-dlp
                    non è disponibile. Vuoi attivarlo e procedere? L'app scaricherà la
                    propria copia{!ffmpegAvailable && <> (con ffmpeg, circa 200 MB)</>} in{' '}
                    <code>%AppData%\RenameMusic</code> e avvierà
                    subito il download {downloadWhat}.
                </p>
            )}
        </ConfirmDialog>
    )
}

export function UninstallYtDlpConfirm({ onCancel, onConfirm, disabled }: {
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Disinstallare yt-dlp?"
            confirmLabel="Disinstalla"
            confirmClassName="danger-solid"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                La copia gestita dall'app in <code>%AppData%\RenameMusic</code> verrà
                <strong> rimossa</strong> (ffmpeg resta). Potrai riscaricarla in qualsiasi momento
                dal tasto accanto a yt-dlp o dal prossimo download di una playlist.
            </p>
        </ConfirmDialog>
    )
}

export function UninstallFFmpegConfirm({ onCancel, onConfirm, disabled }: {
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Disinstallare ffmpeg?"
            confirmLabel="Disinstalla"
            confirmClassName="danger-solid"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                La copia gestita dall'app in <code>%AppData%\RenameMusic\ffmpeg</code> verrà
                <strong> rimossa</strong>. Senza ffmpeg non si possono scaricare playlist in mp3:
                potrai riscaricarlo dal tasto accanto a ffmpeg o dal prossimo download di una
                playlist.
            </p>
        </ConfirmDialog>
    )
}

// DownloadYtDlpConfirm: tasto per scaricare (su Android aggiornare) yt-dlp.
// effectivePath è dove finirà la copia, se il core lo sa già.
export function DownloadYtDlpConfirm({ effectivePath, managed, onCancel, onConfirm, disabled }: {
    effectivePath: string
    managed: boolean
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title={isAndroid ? 'Aggiornare yt-dlp?' : 'Scaricare yt-dlp?'}
            confirmLabel={isAndroid ? 'Aggiorna' : 'Scarica'}
            confirmClassName="accent"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                Verrà scaricata l'ultima versione ufficiale di <strong>yt-dlp</strong> da
                Internet (GitHub){isAndroid ? (
                    <>, al posto di quella integrata nell'app</>
                ) : effectivePath ? (
                    <> in <code>{effectivePath}</code></>
                ) : managed ? (
                    <> in <code>%AppData%\RenameMusic</code></>
                ) : (
                    <> nel percorso indicato</>
                )}. Assicurati di scaricarlo solo da una fonte di cui ti fidi.
            </p>
        </ConfirmDialog>
    )
}

// FFmpegConfirm: download di ffmpeg dal tasto nel pannello ('install') o
// prima del download di una playlist che ne ha bisogno ('playlist').
export function FFmpegConfirm({ reason, downloadWhat, onCancel, onConfirm, disabled }: {
    reason: 'install' | 'playlist'
    downloadWhat: string
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Scaricare ffmpeg?"
            confirmLabel={reason === 'playlist' ? 'Scarica e continua' : 'Scarica'}
            confirmClassName="accent"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                {reason === 'playlist' && (
                    <>Per creare gli mp3 yt-dlp ha bisogno di <strong>ffmpeg</strong>, che non è presente. </>
                )}
                Verrà scaricata l'ultima build ufficiale di <strong>ffmpeg</strong> per
                yt-dlp da Internet (GitHub, circa 200 MB) in{' '}
                <code>%AppData%\RenameMusic\ffmpeg</code>
                {reason === 'playlist' && <>, poi partirà il download {downloadWhat}</>}.
            </p>
        </ConfirmDialog>
    )
}

export function ResetDefaultsConfirm({ onCancel, onConfirm, disabled }: {
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Ripristinare i predefiniti?"
            confirmLabel="Ripristina"
            confirmClassName="danger"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                Regole di rinomina, playlist e modalità semplificata verranno{' '}
                <strong>sostituite</strong> dai valori predefiniti. Le impostazioni attuali andranno
                perse.
            </p>
        </ConfirmDialog>
    )
}

export function SaveDefaultsConfirm({ onCancel, onConfirm, disabled }: {
    onCancel: () => void
    onConfirm: () => void
    disabled: boolean
}) {
    return (
        <ConfirmDialog
            title="Salvare come predefiniti?"
            confirmLabel="Conferma"
            confirmClassName="accent"
            onCancel={onCancel}
            onConfirm={onConfirm}
            disabled={disabled}
        >
            <p>
                I predefiniti attuali verranno <strong>sovrascritti</strong> con le regole di
                rinomina, le playlist e la modalità semplificata correnti. "Ripristina predefiniti"
                userà d'ora in poi questi valori.
            </p>
        </ConfirmDialog>
    )
}

// CrashDialog (solo Android): l'app si è chiusa in modo anomalo e c'è il
// registro dell'errore da condividere. Si chiude solo con una scelta.
export function CrashDialog({ onShare, onDiscard }: { onShare: () => void; onDiscard: () => void }) {
    return (
        <Modal>
            <h3>L'app si è chiusa in modo anomalo</h3>
            <p>
                È stato salvato un file con i dettagli dell'errore. Condividilo con lo
                sviluppatore (per esempio su WhatsApp) per aiutarlo a risolvere il problema.
            </p>
            <div className="modal-actions">
                <button onClick={onDiscard}>Ignora</button>
                <button className="accent" onClick={onShare}>
                    Condividi errore
                </button>
            </div>
        </Modal>
    )
}
