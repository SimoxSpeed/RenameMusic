import { type core, type playlist } from '../api'
import { CloseIcon, LinkIcon, PasteIcon, PlaylistAddIcon, SettingsIcon } from '../icons'
import { Tooltip, Select } from './controls'
import { playlistIdOf, playlistKeyOf } from '../lib/links'

// Voce della scelta della playlist da scaricare: una playlist dell'account
// Google (value "yt:<id>") o una salvata nelle Impostazioni ("pl:<nome>").
export type PlaylistChoice = { value: string; label: string; separator?: boolean }

// playlistChoices elenca prima le playlist dell'account Google, poi (dopo un
// separatore) quelle salvate, senza quelle nascoste nelle loro impostazioni.
// Una salvata che è anche dell'account compare solo tra quelle dell'account
// (le impostazioni sono le stesse: vedi playlistKeyOf).
export function playlistChoices(
    account: core.GooglePlaylistView[],
    saved: playlist.Playlist[],
    prefs: Record<string, playlist.Prefs>,
): PlaylistChoice[] {
    const accountIds = new Set(account.map((p) => p.id))
    const own = account
        .filter((p) => !prefs['yt:' + p.id]?.hidden)
        .map((p) => ({ value: 'yt:' + p.id, label: p.title || p.id }))
    const rest = saved
        .filter((p) => !accountIds.has(playlistIdOf(p.url)) && !prefs[playlistKeyOf(p.url, p.name)]?.hidden)
        .map((p) => ({ value: 'pl:' + p.name, label: p.name }))
    if (own.length > 0 && rest.length > 0) return [...own, { value: 'sep', label: '', separator: true }, ...rest]
    return [...own, ...rest]
}

// PlaylistSelect: la Select per la scelta della playlist da scaricare. Gestisce
// lo stato "vuoto" (nessuna playlist) disabilitando il trigger.
export function PlaylistSelect({
    value,
    options,
    onChange,
    disabled,
}: {
    value: string
    options: PlaylistChoice[]
    onChange: (value: string) => void
    disabled?: boolean
}) {
    const empty = options.length === 0
    return (
        <Select
            className="select-playlist"
            value={value}
            options={options}
            onChange={onChange}
            disabled={disabled || empty}
            placeholder={empty ? 'Nessuna playlist' : 'Seleziona playlist'}
        />
    )
}

// PrefsButton: tasto ⚙ che apre le impostazioni di una playlist (Impostazioni
// > Download). Evidenziato se le impostazioni non sono quelle predefinite
// (summary è il loro riassunto, '' se predefinite).
export function PrefsButton({
    summary,
    onClick,
    disabled,
}: {
    summary: string
    onClick: () => void
    disabled?: boolean
}) {
    return (
        <Tooltip label={summary ? 'Impostazioni della playlist: ' + summary : 'Impostazioni della playlist'}>
            <button
                type="button"
                className={'ghost small prefs-btn' + (summary ? ' is-on' : '')}
                onClick={onClick}
                disabled={disabled}
                aria-label="Impostazioni della playlist"
            >
                <SettingsIcon />
            </button>
        </Tooltip>
    )
}

// LinkField: campo per il link da scaricare (di solito un singolo video), sopra
// la scelta della playlist, con il tasto per svuotarlo. Invio avvia il download.
// onAddToPlaylist, se c'è (account Google collegato e link di un video), mostra
// anche il tasto per aggiungere il video a una playlist dell'account. onPaste,
// se c'è (solo Android), mostra a campo vuoto il tasto "Incolla" al posto
// della ✕: gli appunti si leggono solo con quel tocco.
export function LinkField({
    value,
    onChange,
    onSubmit,
    onAddToPlaylist,
    onPaste,
    disabled,
}: {
    value: string
    onChange: (value: string) => void
    onSubmit: () => void
    onAddToPlaylist?: () => void
    onPaste?: () => void
    disabled?: boolean
}) {
    const canAdd = !!onAddToPlaylist && value !== '' && !disabled
    const canPaste = !!onPaste && value === ''
    return (
        <div className={'link-field' + (canAdd ? ' has-add' : '') + (canPaste ? ' has-paste' : '')}>
            <span className="link-field-icon" aria-hidden="true">
                <LinkIcon />
            </span>
            <input
                type="url"
                inputMode="url"
                autoComplete="off"
                spellCheck={false}
                value={value}
                placeholder="Incolla il link di un video"
                aria-label="Link da scaricare"
                onChange={(e) => onChange(e.target.value)}
                onKeyDown={(e) => {
                    if (e.key !== 'Enter') return
                    e.preventDefault()
                    onSubmit()
                }}
                disabled={disabled}
            />
            {canAdd && (
                <Tooltip label="Aggiungi il video a una playlist del tuo account YouTube">
                    <button
                        type="button"
                        className="link-field-add"
                        onClick={onAddToPlaylist}
                        aria-label="Aggiungi il video a una playlist"
                    >
                        <PlaylistAddIcon />
                    </button>
                </Tooltip>
            )}
            {value !== '' && !disabled && (
                <button
                    type="button"
                    className="link-field-clear"
                    onClick={() => onChange('')}
                    aria-label="Svuota il link"
                >
                    <CloseIcon />
                </button>
            )}
            {canPaste && (
                <button
                    type="button"
                    className="link-field-paste"
                    onClick={onPaste}
                    disabled={disabled}
                    aria-label="Incolla il link dagli appunti"
                >
                    <PasteIcon />
                    Incolla
                </button>
            )}
        </div>
    )
}
