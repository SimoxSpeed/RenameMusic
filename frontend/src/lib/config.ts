import { type rules, type playlist } from '../api'

export function cloneConfig(cfg: rules.Config): rules.Config {
    return {
        startFolder: cfg.startFolder,
        supportedExtensions: [...(cfg.supportedExtensions ?? [])],
        occurrenciesToRemove: [...(cfg.occurrenciesToRemove ?? [])],
        occurrenciesToReplaceWithFt: [...(cfg.occurrenciesToReplaceWithFt ?? [])],
        ftAlias: cfg.ftAlias,
        replacements: (cfg.replacements ?? []).map((r) => ({ from: r.from, to: r.to, scope: r.scope })),
        artistExceptions: [...(cfg.artistExceptions ?? [])],
        simpleMode: !!cfg.simpleMode,
        musicBrainz: !!cfg.musicBrainz,
    } as rules.Config
}

// comparableConfig / comparablePlaylists: forma confrontabile di regole e
// playlist, pulite come le pulisce il core al salvataggio (normalizeConfig,
// cleanPlaylists): righe vuote e spazi ai bordi non contano come modifiche. La
// cartella è esclusa, perché il core la gestisce a parte.
export function comparableConfig(cfg: rules.Config): string {
    const list = (values?: string[]) => (values ?? []).filter((v) => v.trim() !== '')
    const c = cloneConfig(cfg)
    return JSON.stringify({
        ...c,
        startFolder: undefined,
        supportedExtensions: list(c.supportedExtensions),
        occurrenciesToRemove: list(c.occurrenciesToRemove),
        occurrenciesToReplaceWithFt: list(c.occurrenciesToReplaceWithFt),
        artistExceptions: list(c.artistExceptions),
        ftAlias: (c.ftAlias ?? '').trim(),
        replacements: c.replacements
            .filter((r) => (r.from ?? '').trim() !== '')
            .map((r) => ({ ...r, scope: r.scope || undefined })),
    })
}

export function comparablePlaylists(list: playlist.Playlist[]): string {
    return JSON.stringify(
        list
            .map((p) => ({ name: p.name.trim(), url: p.url.trim() }))
            .filter((p) => p.name !== '' && p.url !== ''),
    )
}

// prefsSummary riassume le impostazioni di una playlist ('' se predefinite).
export function prefsSummary(p?: playlist.Prefs): string {
    const parts: string[] = []
    if (p?.hidden) parts.push('Nascosta')
    if (p?.afterDownload === 'remove') parts.push('Si svuota dopo il download')
    if (p?.afterDownload === 'copy') {
        const verb = p.moveOnCopy ? 'si spostano' : 'si aggiungono'
        parts.push(`Dopo il download i brani ${verb} in «${p.copyToTitle || p.copyTo}»`)
    }
    return parts.join(' · ')
}

