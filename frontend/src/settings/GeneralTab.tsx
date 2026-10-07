import { isAndroid, type rules } from '../api'
import { CheckOption, Tooltip } from '../components/controls'
import { MissingFolderIcon } from '../components/folders'
import { FolderOpenIcon } from '../icons'

// FolderSettingsProps: cartelle di partenza/destinazione e opzioni di
// conversione. Stanno sempre qui (scheda Generale), in qualunque modalità: si
// applicano subito e non fanno parte dei predefiniti. storageGranted (Android)
// dice se l'app può accedere ai file, senza il quale non si sceglie nulla.
export type FolderSettingsProps = {
    folder: string
    folderMissing: boolean
    destFolder: string
    destMissing: boolean
    destSameAsSource: boolean
    deleteOriginals: boolean
    storageGranted: boolean
    busy: boolean
    onOpenFolder: (path: string) => void
    onChooseFolder: () => void
    onChooseDestination: () => void
    onDestSameAsSourceChange: (checked: boolean) => void
    onDeleteOriginalsChange: (checked: boolean) => void
}

function FolderSettings(props: FolderSettingsProps) {
    const { folder, folderMissing, destFolder, destMissing, destSameAsSource, deleteOriginals, storageGranted, busy } =
        props
    return (
        <>
            <div className="field-group">
                <span className="field-label">Cartella di partenza</span>
                <div className="toolbar">
                    <div className="folder-path">{folder || 'Nessuna cartella selezionata'}</div>
                    {folderMissing && <MissingFolderIcon label="Cartella di partenza" />}
                    {!isAndroid && (
                        <Tooltip label="Apri la cartella in Esplora risorse">
                            <button
                                className="ghost with-icon"
                                onClick={() => props.onOpenFolder(folder)}
                                disabled={busy || !folder || folderMissing}
                            >
                                <span className="btn-icon">
                                    <FolderOpenIcon />
                                </span>
                                Apri
                            </button>
                        </Tooltip>
                    )}
                    <button className="primary" onClick={props.onChooseFolder} disabled={busy || !storageGranted}>
                        Scegli cartella
                    </button>
                </div>
            </div>

            {!destSameAsSource && (
                <div className="field-group">
                    <span className="field-label">Cartella di destinazione</span>
                    <div className="toolbar">
                        <div className="folder-path">{destFolder || 'Nessuna destinazione selezionata'}</div>
                        {destMissing && <MissingFolderIcon label="Cartella di destinazione" />}
                        {!isAndroid && (
                            <Tooltip label="Apri la cartella in Esplora risorse">
                                <button
                                    className="ghost with-icon"
                                    onClick={() => props.onOpenFolder(destFolder)}
                                    disabled={busy || !destFolder || destMissing}
                                >
                                    <span className="btn-icon">
                                        <FolderOpenIcon />
                                    </span>
                                    Apri
                                </button>
                            </Tooltip>
                        )}
                        <button
                            className="primary"
                            onClick={props.onChooseDestination}
                            disabled={busy || !storageGranted}
                        >
                            Scegli cartella
                        </button>
                    </div>
                </div>
            )}

            <div className="options">
                <CheckOption
                    label="Destinazione uguale alla cartella di partenza"
                    info="Se attiva, i file convertiti vengono scritti nella stessa cartella dei file originali. Se disattivata puoi scegliere una cartella di destinazione separata."
                    checked={destSameAsSource}
                    onChange={props.onDestSameAsSourceChange}
                    disabled={busy}
                />

                <CheckOption
                    label="Eliminazione file originali"
                    info="Quando attiva, dopo la conversione i file di partenza vengono eliminati definitivamente dal disco. Quando disattivata, i nuovi file convertiti vengono scritti senza toccare gli originali."
                    checked={deleteOriginals}
                    onChange={props.onDeleteOriginalsChange}
                    disabled={busy}
                />
            </div>
        </>
    )
}

// GeneralTab: scheda Generale. In cima le preferenze salvate con le regole
// (modalità semplificata, ricerca su MusicBrainz: draft), sotto le cartelle.
export function GeneralTab({
    draft,
    onDraftChange,
    ...folders
}: FolderSettingsProps & {
    draft: rules.Config
    onDraftChange: (draft: rules.Config) => void
}) {
    const { busy } = folders
    return (
        <section className="settings">
            <CheckOption
                label="Modalità semplificata"
                info="Quando attiva, la schermata principale mostra solo il download (da un link o da una playlist): «Scarica e converti» scarica i brani e li converte subito (nomi e tag), senza anteprima."
                checked={!!draft.simpleMode}
                onChange={(checked) => onDraftChange({ ...draft, simpleMode: checked } as rules.Config)}
                disabled={busy}
            />
            <CheckOption
                label="Cerca titolo e artista su MusicBrainz"
                info="Quando dal nome di una traccia non si capiscono titolo e artista (o la segni da rivedere), il popup propone il nome trovato su musicbrainz.org al posto di quello attuale. Per la ricerca il nome della traccia viene inviato a MusicBrainz."
                checked={!!draft.musicBrainz}
                onChange={(checked) => onDraftChange({ ...draft, musicBrainz: checked } as rules.Config)}
                disabled={busy}
            />
            <hr className="settings-divider" />
            <div className="settings-folders">
                <FolderSettings {...folders} />
            </div>
        </section>
    )
}
