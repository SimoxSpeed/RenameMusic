import { AlertIcon } from '../icons'

// ExtChip mostra il formato del file in stile blu. I file trattati sono sempre
// mp3 (nessuna conversione tra formati diversi), quindi mostriamo semplicemente
// l'estensione: serve solo perché l'estensione non è mai visibile nei nomi.
export function ExtChip({ ext }: { ext: string }) {
    if (!ext) return null
    return <span className="ext-chip ext-same">{ext}</span>
}

// CurrentField mostra una riga "etichetta: valore" nella colonna File attuale
// (nome/titolo/artista impilati in una sola cella). Se il valore sta per
// cambiare compare sbarrato e attenuato (stessa classe .old-name della colonna
// Nome); se manca del tutto (tag non presente) mostra un placeholder neutro.
export function CurrentField({ label, value, changed }: { label: string; value: string; changed: boolean }) {
    return (
        <div className="current-line">
            <span className="current-label">{label}:</span>{' '}
            {!value ? (
                <span className="muted-dash">nessun tag</span>
            ) : changed ? (
                <s className="old-name">{value}</s>
            ) : (
                value
            )}
        </div>
    )
}

// ErrorLabel: chip rosso "Errore" nella colonna esito. Non è un elemento
// interattivo (niente click/animazione): serve solo a mostrare un tooltip al
// passaggio del mouse o al focus da tastiera, così la tabella resta compatta
// ma il dettaglio è a un hover di distanza.
export function ErrorLabel({ message }: { message: string }) {
    return (
        <span className="result-error" tabIndex={0} aria-label={'Errore: ' + message}>
            <AlertIcon />
            Errore
            <span className="result-error-tip" role="tooltip">{message}</span>
        </span>
    )
}
