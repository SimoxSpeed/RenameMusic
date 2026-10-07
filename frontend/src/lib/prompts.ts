import { type core } from '../api'

// Valori-sentinella dei tag "sconosciuti" (devono combaciare con le costanti
// parser.UnknownTitle/UnknownArtist lato Go): identificano una traccia il cui
// nome non permette di dedurre titolo/artista.
export const UNKNOWN_TITLE = 'Titolo Sconosciuto'
export const UNKNOWN_ARTIST = 'Artista Sconosciuto'

// TagPrompt: richiesta (payload dell'evento process:needTagInput) di correggere
// una traccia i cui tag risulterebbero sconosciuti. La UI ne accoda una alla
// volta mostrando un popup con `originalBase` modificabile.
export type TagPrompt = {
    path: string
    originalBase: string
    ext: string
    title: string
    artist: string
    // review: traccia selezionata nell'anteprima per essere rivista (non ha tag
    // sconosciuti); previewBase è il nome proposto, da cui parte il popup.
    review: boolean
    previewBase: string
}

// foldText porta un testo in minuscolo e senza accenti, per cercarlo
// ("beyonce" trova "Beyoncé").
export function foldText(s: string): string {
    return s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()
}

// PromptSearch: ricerca su MusicBrainz per il popup di una TagPrompt. names
// sono i nomi trovati, dal più probabile; error il motivo se non è riuscita;
// skipped se l'utente non l'ha aspettata.
export type PromptSearch = {
    loading: boolean
    names: string[]
    error: string
    skipped?: boolean
}

// promptsOf estrae dalla risposta di una conversione le tracce da confermare
// (tag sconosciuti o selezionate da rivedere), che la UI risolve una alla
// volta col popup.
export function promptsOf(resp: core.ActionResponse): TagPrompt[] {
    return (resp.prompts ?? []).map((p) => ({
        path: p.path,
        originalBase: p.originalBase,
        ext: p.ext,
        title: p.title,
        artist: p.artist,
        review: !!p.review,
        previewBase: p.previewBase ?? '',
    }))
}
