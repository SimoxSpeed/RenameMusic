import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { isAndroid } from '../api'
import { CaretIcon, CloseIcon, InfoCircleIcon } from '../icons'

// ChipList: elenco di voci modificabile come etichette affiancate (regole a
// elenco delle Impostazioni). Invio o l'uscita dal campo aggiungono il testo
// scritto, ✕ toglie una voce; incollando più righe ogni riga diventa una voce.
// Le voci restano come scritte (gli spazi ai bordi possono contare, es. " x "):
// si scartano solo quelle vuote e i doppioni. Non è dentro una <label>, perché
// un clic sull'etichetta attiverebbe il primo ✕. label dà il nome al campo per
// gli screen reader; caption, se c'è, è l'etichetta visibile sopra l'elenco.
export function ChipList({ values, onChange, label, caption, placeholder, disabled }: {
    values: string[]
    onChange: (values: string[]) => void
    label: string
    caption?: string
    placeholder?: string
    disabled?: boolean
}) {
    const [text, setText] = useState('')

    function add(items: string[]) {
        const next = [...values]
        for (const v of items) {
            if (v.trim() !== '' && !next.includes(v)) next.push(v)
        }
        if (next.length !== values.length) onChange(next)
        setText('')
    }

    return (
        <div className="field-group">
            {caption && <span className="field-label">{caption}</span>}
            <div className="chip-list">
                {values.map((v, i) => (
                    <span className="chip" key={i}>
                        <span className="chip-text">{v}</span>
                        <button
                            type="button"
                            className="chip-remove"
                            onClick={() => onChange(values.filter((_, j) => j !== i))}
                            disabled={disabled}
                            aria-label={'Rimuovi ' + v.trim()}
                        >
                            <CloseIcon />
                        </button>
                    </span>
                ))}
                <input
                    type="text"
                    className="chip-input"
                    placeholder={placeholder ?? 'Aggiungi e premi Invio'}
                    aria-label={label}
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                            e.preventDefault()
                            add([text])
                        }
                    }}
                    onBlur={() => add([text])}
                    onPaste={(e) => {
                        const pasted = e.clipboardData.getData('text')
                        if (!pasted.includes('\n')) return
                        e.preventDefault()
                        add(pasted.split(/\r?\n/))
                    }}
                    disabled={disabled}
                />
            </div>
        </div>
    )
}

// InfoIcon: pulsante con tooltip custom. \u00c8 un <button> per essere focusabile
// da tastiera e per catturare il click impedendo che tocchi la <label> genitore
// (altrimenti cliccare la "i" attiverebbe la checkbox associata).
// Su Android il fumetto non segue hover/focus (su touch restano "attaccati"):
// un tocco lo apre, un nuovo tocco o un tocco altrove lo chiude.
export function InfoIcon({ text }: { text: string }) {
    const [open, setOpen] = useState(false)
    const ref = useRef<HTMLButtonElement>(null)

    useEffect(() => {
        if (!open) return
        const closeOutside = (e: PointerEvent) => {
            if (!ref.current?.contains(e.target as Node)) setOpen(false)
        }
        document.addEventListener('pointerdown', closeOutside)
        return () => document.removeEventListener('pointerdown', closeOutside)
    }, [open])

    return (
        <button
            ref={ref}
            type="button"
            className={'info-icon' + (open ? ' is-open' : '')}
            aria-label={text}
            aria-expanded={isAndroid ? open : undefined}
            onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                if (isAndroid) setOpen((o) => !o)
            }}
        >
            <InfoCircleIcon />
            <span className="info-tooltip" role="tooltip">{text}</span>
        </button>
    )
}

// CheckOption: checkbox con testo e icona "i". L'icona sta nel flusso del
// testo, così segue l'ultima parola anche quando il testo va a capo (dopo una
// <label> a capo finirebbe sul bordo destro). Non può stare dentro la <label>
// (un bottone in una label non è valido): la label è collegata alla casella
// con htmlFor. L'ultima parola è tenuta insieme all'icona (check-tail), così
// l'icona non va mai a capo da sola: per questo le label sono due, entrambe
// collegate alla stessa casella.
export function CheckOption({ label, info, checked, onChange, disabled, className }: {
    label: string
    info: string
    checked: boolean
    onChange: (checked: boolean) => void
    disabled?: boolean
    className?: string
}) {
    const id = useId()
    const cut = label.lastIndexOf(' ') + 1
    const head = label.slice(0, cut)
    const tail = label.slice(cut)
    return (
        <div className={'check' + (className ? ' ' + className : '')}>
            <span className="check-box">
                <input
                    id={id}
                    type="checkbox"
                    checked={checked}
                    onChange={(e) => onChange(e.target.checked)}
                    disabled={disabled}
                />
            </span>
            <span className="check-text">
                {head && <label htmlFor={id} className="check-label">{head}</label>}
                <span className="check-tail">
                    <label htmlFor={id} className="check-label">{tail}</label>
                    <InfoIcon text={info} />
                </span>
            </span>
        </div>
    )
}

// Tooltip avvolge un elemento e mostra un fumetto informativo moderno (stesso
// stile del tooltip "i"/scorciatoie) su hover o focus. Essendo un wrapper, il
// fumetto compare anche quando l'elemento interno è disabilitato (i bottoni
// disabilitati non ricevono hover, ma il wrapper sì): utile per spiegare PERCHÉ
// un'azione non è disponibile.
export function Tooltip({ label, children }: { label: string; children: ReactNode }) {
    if (!label) return <>{children}</>
    return (
        <span className="tip">
            {children}
            <span className="info-tooltip" role="tooltip">{label}</span>
        </span>
    )
}

// Select: dropdown custom riutilizzabile. Il <select> nativo apre una lista
// disegnata dal WebView (aspetto "di sistema", non stilabile): qui la
// sostituiamo con un trigger + lista nostra (angoli arrotondati, ombra, colori
// del tema, caret coerente) usati in modo uniforme in tutta l'app — download
// playlist e select delle impostazioni. Chiude su click-fuori ed Esc.
export function Select({
    value,
    options,
    onChange,
    disabled,
    placeholder,
    className,
}: {
    value: string
    // separator: riga divisoria tra gruppi di voci, non selezionabile.
    options: { value: string; label: string; separator?: boolean }[]
    onChange: (value: string) => void
    disabled?: boolean
    placeholder?: string
    className?: string
}) {
    const [open, setOpen] = useState(false)
    const wrapRef = useRef<HTMLDivElement>(null)

    // I listener (click-fuori ed Esc) vivono solo mentre la lista è aperta.
    useEffect(() => {
        if (!open) return
        function onDown(e: MouseEvent) {
            if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false)
        }
        function onKey(e: KeyboardEvent) {
            if (e.key === 'Escape') setOpen(false)
        }
        window.addEventListener('mousedown', onDown)
        window.addEventListener('keydown', onKey)
        return () => {
            window.removeEventListener('mousedown', onDown)
            window.removeEventListener('keydown', onKey)
        }
    }, [open])

    const selected = options.find((o) => o.value === value)
    const label = selected ? selected.label : placeholder ?? ''

    return (
        <div className={'select-wrap' + (className ? ' ' + className : '')} ref={wrapRef}>
            <button
                type="button"
                className="select-trigger"
                aria-haspopup="listbox"
                aria-expanded={open}
                disabled={disabled}
                onClick={() => setOpen((o) => !o)}
            >
                <span className="select-value">{label}</span>
                <span className={'select-caret' + (open ? ' is-open' : '')}>
                    <CaretIcon />
                </span>
            </button>
            {open && options.length > 0 && (
                <ul className="select-menu" role="listbox">
                    {options.map((o) => o.separator ? (
                        <li key={o.value} role="separator" className="select-separator" />
                    ) : (
                        <li
                            key={o.value}
                            role="option"
                            aria-selected={o.value === value}
                            className={'select-option' + (o.value === value ? ' is-selected' : '')}
                            onClick={() => {
                                onChange(o.value)
                                setOpen(false)
                            }}
                        >
                            {o.label}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    )
}

// Collapse: contenitore che con `collapsed` si chiude fino a sparire (poi resta
// display: none) e si riapre, animato lungo la direzione della riga flex che lo
// contiene: in larghezza se i comandi sono affiancati, in altezza se sono
// impilati. Si anima una misura esplicita, presa dall'elemento vero, più il
// margine che si mangia il gap: così il fratello che ne prende il posto (regole
// con [data-away] nel CSS) cresce in modo continuo, senza scatti. data-away c'è
// finché l'elemento è chiuso o in movimento. Stili e attributo si impostano a
// mano sul nodo, fuori da React, perché cambiano a ogni frame. onSettled avvisa
// quando l'elemento è fermo (aperto o chiuso).
export function Collapse({
    collapsed,
    className,
    onSettled,
    children,
}: {
    collapsed: boolean
    className: string
    onSettled?: () => void
    children: ReactNode
}) {
    const ref = useRef<HTMLDivElement>(null)
    const first = useRef(true)
    const onSettledRef = useRef(onSettled)
    onSettledRef.current = onSettled
    useLayoutEffect(() => {
        const el = ref.current
        if (!el) return
        const initial = first.current
        first.current = false
        const settle = () => {
            el.style.flex = ''
            el.style.overflow = ''
            if (collapsed) el.style.display = 'none'
            else delete el.dataset.away
            onSettledRef.current?.()
        }
        if (initial || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
            el.getAnimations().forEach((a) => a.cancel())
            if (collapsed) el.dataset.away = ''
            else el.style.display = ''
            settle()
            return
        }

        const row = el.parentElement ? getComputedStyle(el.parentElement) : null
        const vertical = !!row && row.flexDirection.startsWith('column')
        const size = vertical ? 'height' : 'width'
        const margin = vertical ? 'marginBottom' : 'marginRight'
        const gap = (row && parseFloat(vertical ? row.rowGap : row.columnGap)) || 0

        // Da dove si parte: anche da metà di un'animazione nel verso opposto.
        const gone = el.style.display === 'none'
        const style = getComputedStyle(el)
        const from = gone
            ? { size: 0, margin: -gap, opacity: 0 }
            : { size: el.getBoundingClientRect()[size], margin: parseFloat(style[margin]) || 0, opacity: parseFloat(style.opacity) }
        el.getAnimations().forEach((a) => a.cancel())

        // Dove si arriva: da aperto è la misura naturale, senza gli stili
        // dell'animazione e senza che il fratello si allarghi.
        let to = 0
        if (!collapsed) {
            el.style.display = ''
            el.style.flex = ''
            el.style.overflow = ''
            delete el.dataset.away
            to = el.getBoundingClientRect()[size]
        }
        el.dataset.away = ''
        el.style.display = ''
        el.style.flex = '0 0 auto'
        el.style.overflow = 'hidden'
        // Chiudendo il contenuto svanisce presto; riaprendo compare quando c'è
        // già un po' di spazio.
        const fade = collapsed ? { opacity: 0, offset: 0.6 } : { opacity: from.opacity, offset: 0.35 }
        const anim = el.animate(
            [
                { [size]: from.size + 'px', [margin]: from.margin + 'px', opacity: from.opacity },
                fade,
                { [size]: to + 'px', [margin]: (collapsed ? -gap : 0) + 'px', opacity: collapsed ? 0 : 1 },
            ],
            { duration: 300, easing: 'cubic-bezier(0.4, 0, 0.2, 1)' },
        )
        anim.onfinish = settle
    }, [collapsed])
    return (
        <div ref={ref} className={className} aria-hidden={collapsed || undefined}>
            {children}
        </div>
    )
}

// OpProgress è la barra di avanzamento delle operazioni lunghe: `percent`
// (0-100) riempie la barra; null => totale non noto, si mostra solo l'etichetta.
export function OpProgress({ percent, label, className }: { percent: number | null; label: string; className?: string }) {
    return (
        <div className={'op-progress' + (className ? ' ' + className : '')}>
            {percent !== null && (
                <div
                    className="op-progress-track"
                    role="progressbar"
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={percent}
                >
                    <div className="op-progress-fill" style={{ width: percent + '%' }} />
                </div>
            )}
            <span className="op-progress-label">{label}</span>
        </div>
    )
}
