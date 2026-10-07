// linkInText trova un link http(s) in un testo incollato (condividendo
// dall'app di YouTube il link arriva a volte dopo altro testo): prima uno di
// YouTube, altrimenti il primo che c'è; '' se non ce ne sono.
export function linkInText(text: string): string {
    const links = text.match(/https?:\/\/\S+/gi) ?? []
    return links.find((l) => youtubeUrl(l) !== null) ?? links[0] ?? ''
}

// youtubeUrl legge un link di YouTube (anche m., music., youtu.be); null se
// non lo è. Stessi controlli di youtube.PlaylistID/VideoID lato Go.
export function youtubeUrl(link: string): URL | null {
    try {
        const u = new URL(link.trim())
        const host = u.hostname.toLowerCase().replace(/^www\./, '')
        return host === 'youtube.com' || host.endsWith('.youtube.com') || host === 'youtu.be' ? u : null
    } catch {
        return null
    }
}

// playlistIdOf: ID della playlist di un link di YouTube ('' se non ne ha).
export function playlistIdOf(link: string): string {
    return youtubeUrl(link)?.searchParams.get('list') ?? ''
}

// playlistKeyOf: chiave delle impostazioni di una playlist salvata, come
// playlistKey lato Go: quella della playlist di YouTube del link, o il nome.
export function playlistKeyOf(url: string, name: string): string {
    const id = playlistIdOf(url)
    return id ? 'yt:' + id : 'pl:' + name.trim()
}

// isVideoLink: il link punta a un video di YouTube (watch?v=, youtu.be/, shorts/).
export function isVideoLink(link: string): boolean {
    const u = youtubeUrl(link)
    if (!u) return false
    const id =
        u.hostname.toLowerCase() === 'youtu.be'
            ? u.pathname.split('/')[1]
            : (u.searchParams.get('v') ?? u.pathname.match(/^\/(?:shorts|live|embed)\/([^/]+)/)?.[1])
    return /^[A-Za-z0-9_-]{11}$/.test(id ?? '')
}
