import { type core } from '../api'

// splitName separa il nome file dalla sua estensione (parte dopo l'ultimo punto).
// Restituisce base senza estensione ed estensione senza punto. Usato per NON
// mostrare mai l'estensione nei nomi file: quella viaggia in un chip a parte.
export function splitName(full: string): { base: string; ext: string } {
    const idx = full.lastIndexOf('.')
    if (idx <= 0 || idx === full.length - 1) return { base: full, ext: '' }
    return { base: full.slice(0, idx), ext: full.slice(idx + 1).toLowerCase() }
}

// tagChanged confronta il tag ID3 attuale con quello che verrebbe scritto.
export function tagChanged(current: string | undefined, expected: string | undefined): boolean {
    return (current ?? '') !== (expected ?? '')
}

// fileWillChange indica se un file subirà una qualsiasi modifica: il nome
// cambia, oppure (per gli MP3) cambia il titolo o l'artista scritto nei tag.
// Usata sia dal filtro "Solo da modificare" sia dai badge di stato.
export function fileWillChange(f: core.FileView): boolean {
    const nameChanged = splitName(f.name).base !== splitName(f.preview).base
    const tagsChanged = !!f.mp3 && (tagChanged(f.title, f.titlePreview) || tagChanged(f.artist, f.artistPreview))
    return nameChanged || tagsChanged
}
