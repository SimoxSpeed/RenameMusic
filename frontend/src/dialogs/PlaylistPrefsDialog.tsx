import { type Dispatch, type SetStateAction } from 'react'
import { type core, type playlist } from '../api'
import { CheckOption, Select } from '../components/controls'
import { Modal } from '../components/Modal'
import { TrashIcon } from '../icons'

// PlaylistPrefsTarget: playlist di cui si aprono le impostazioni. key come
// StateResponse.playlistPrefs; ownId è il suo ID se è dell'account (allora si
// può modificare su YouTube: svuotarla o toglierne i brani).
export type PlaylistPrefsTarget = { key: string; title: string; ownId: string }

// PlaylistPrefsDialog: impostazioni di una playlist della scelta del download
// (tasto ⚙). draft sono le impostazioni in modifica, salvate solo con «Salva»;
// connected dice se l'account Google è collegato, accountPlaylists sono le
// sue playlist (le possibili destinazioni della copia). onEmpty apre la
// conferma di «Svuota ora».
export function PlaylistPrefsDialog({
    target,
    connected,
    accountPlaylists,
    draft,
    setDraft,
    busy,
    onClose,
    onSave,
    onEmpty,
}: {
    target: PlaylistPrefsTarget
    connected: boolean
    accountPlaylists: core.GooglePlaylistView[]
    draft: playlist.Prefs
    setDraft: Dispatch<SetStateAction<playlist.Prefs>>
    busy: boolean
    onClose: () => void
    onSave: () => void
    onEmpty: () => void
}) {
    const canRemove = connected && target.ownId !== ''
    const selfId = target.key.startsWith('yt:') ? target.key.slice(3) : ''
    const targets = accountPlaylists
        .filter((p) => p.id !== selfId)
        .map((p) => ({ value: p.id, label: p.title || p.id }))
    const after = draft.afterDownload ?? ''
    // Scegliendo l'aggiunta a un'altra playlist, per una playlist
    // dell'account si propone di spostare (togliere anche dall'origine).
    const setAfter = (value: string) =>
        setDraft((d) => ({
            ...d,
            afterDownload: value,
            moveOnCopy: value === 'copy' && d.moveOnCopy === undefined ? canRemove : d.moveOnCopy,
        }))
    return (
        <Modal onClose={onClose} wide>
            <h3>Impostazioni di «{target.title}»</h3>

            <CheckOption
                label="Nascondi dalla tendina"
                info="La playlist non compare più nella scelta della playlist da scaricare. Resta qui nelle Impostazioni, da dove puoi farla ricomparire."
                checked={!!draft.hidden}
                onChange={(checked) => setDraft((d) => ({ ...d, hidden: checked }))}
            />

            <fieldset className="prefs-group">
                <legend>Dopo il download</legend>
                <label className="radio-option">
                    <input type="radio" name="after" checked={after === ''} onChange={() => setAfter('')} />
                    <span>Non fare nulla</span>
                </label>
                <label className={'radio-option' + (canRemove ? '' : ' is-disabled')}>
                    <input
                        type="radio"
                        name="after"
                        checked={after === 'remove'}
                        disabled={!canRemove && after !== 'remove'}
                        onChange={() => setAfter('remove')}
                    />
                    <span>
                        Togli i brani scaricati dalla playlist su YouTube
                        <small>
                            {canRemove
                                ? 'La playlist fa da coda: restano solo i brani ancora da scaricare.'
                                : connected
                                  ? 'Solo per le playlist del tuo account.'
                                  : 'Collega l\'account Google per usarlo.'}
                        </small>
                    </span>
                </label>
                <label className={'radio-option' + (connected ? '' : ' is-disabled')}>
                    <input
                        type="radio"
                        name="after"
                        checked={after === 'copy'}
                        disabled={!connected && after !== 'copy'}
                        onChange={() => setAfter('copy')}
                    />
                    <span>
                        Aggiungi i brani scaricati a un'altra playlist
                        <small>
                            {connected
                                ? 'In fondo alla playlist scelta, senza doppioni.'
                                : 'Collega l\'account Google per usarlo.'}
                        </small>
                    </span>
                </label>
                {after === 'copy' && (
                    <Select
                        className="prefs-target"
                        value={draft.copyTo ?? ''}
                        options={targets}
                        onChange={(id) =>
                            setDraft((d) => ({
                                ...d,
                                copyTo: id,
                                copyToTitle: targets.find((t) => t.value === id)?.label ?? id,
                            }))
                        }
                        disabled={targets.length === 0}
                        placeholder={targets.length === 0 ? 'Nessun\'altra playlist nel tuo account' : 'Scegli la playlist'}
                    />
                )}
                {after === 'copy' && (
                    <CheckOption
                        className="prefs-move"
                        label="Togli anche dalla playlist di origine"
                        info={
                            canRemove
                                ? 'I brani si spostano: dopo essere stati aggiunti alla playlist scelta vengono tolti da questa. Se l\'aggiunta non riesce, restano qui.'
                                : 'Solo per le playlist del tuo account: le altre non si possono modificare.'
                        }
                        checked={canRemove && !!draft.moveOnCopy}
                        onChange={(checked) => setDraft((d) => ({ ...d, moveOnCopy: checked }))}
                        disabled={!canRemove}
                    />
                )}
            </fieldset>

            {target.ownId !== '' && (
                <fieldset className="prefs-group">
                    <legend>Svuota la playlist</legend>
                    <div className="prefs-empty">
                        <span>Toglie subito da YouTube tutti i brani della playlist, anche quelli non scaricati.</span>
                        <button
                            className="ghost small danger with-icon"
                            onClick={onEmpty}
                            disabled={busy}
                        >
                            <span className="btn-icon"><TrashIcon /></span>
                            Svuota ora
                        </button>
                    </div>
                </fieldset>
            )}

            <div className="modal-actions">
                <button onClick={onClose} disabled={busy}>
                    Annulla
                </button>
                <button
                    className="primary"
                    onClick={onSave}
                    disabled={busy || (after === 'copy' && !draft.copyTo)}
                >
                    Salva
                </button>
            </div>
        </Modal>
    )
}
