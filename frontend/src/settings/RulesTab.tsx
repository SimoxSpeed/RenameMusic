import { type ReactNode } from 'react'
import { type rules } from '../api'
import { ChipList } from '../components/controls'
import { ChevronIcon } from '../icons'

// RuleListKey: regole a elenco di rules.Config, modificate con ChipList.
export type RuleListKey =
    'supportedExtensions' | 'occurrenciesToRemove' | 'occurrenciesToReplaceWithFt' | 'artistExceptions'

// Ambiti delle sostituzioni Da → A, ognuno con il suo gruppo nella scheda
// Regole (scope come in rules.Scope: vuoto = tutto il nome).
const REPLACEMENT_SCOPES = [
    { scope: '', label: 'Tutto il nome', hint: 'Su tutto il nome del file' },
    { scope: 'artist', label: 'Solo artista', hint: 'Solo sulla parte prima di « - »' },
    { scope: 'title', label: 'Solo titolo', hint: 'Solo sulla parte dopo « - »' },
]

// RuleGroup: card richiudibile di una categoria di regole (scheda Regole). Il
// titolo porta descrizione e numero di voci, così anche da chiusa si capisce
// cosa contiene; tone è il colore della categoria (bordo e contatore). Parte
// chiusa: le card chiuse fanno da indice della scheda.
function RuleGroup({
    title,
    hint,
    count,
    tone,
    children,
}: {
    title: string
    hint: string
    count: number
    tone: 'red' | 'blue' | 'green' | 'gray' | 'yellow'
    children: ReactNode
}) {
    return (
        <details className={'rule-group tone-' + tone}>
            <summary>
                <ChevronIcon />
                <span className="rule-group-title">{title}</span>
                <span className="rule-count">{count}</span>
                {hint && <span className="rule-hint">{hint}</span>}
            </summary>
            <div className="rule-group-body">{children}</div>
        </details>
    )
}

// RulesTab: scheda Regole, le regole di normalizzazione in modifica (draft).
// Le sostituzioni sono divise per ambito ma restano un unico elenco: index è
// la posizione nell'elenco completo.
export function RulesTab({
    draft,
    busy,
    onListChange,
    onFtAliasChange,
    onReplacementChange,
    onAddReplacement,
    onRemoveReplacement,
}: {
    draft: rules.Config
    busy: boolean
    onListChange: (key: RuleListKey, values: string[]) => void
    onFtAliasChange: (value: string) => void
    onReplacementChange: (index: number, field: 'from' | 'to', value: string) => void
    onAddReplacement: (scope: string) => void
    onRemoveReplacement: (index: number) => void
}) {
    return (
        <section className="settings">
            {/* Una card richiudibile per categoria: da chiuse fanno
                da indice (titolo, descrizione e numero di voci). */}
            <div className="rule-groups">
                <RuleGroup
                    title="Pulizia del nome"
                    hint="Testi tolti dal nome, ad esempio «(Official Video)»"
                    count={(draft.occurrenciesToRemove ?? []).length}
                    tone="red"
                >
                    <ChipList
                        label="Testi da rimuovere"
                        values={draft.occurrenciesToRemove ?? []}
                        onChange={(v) => onListChange('occurrenciesToRemove', v)}
                        disabled={busy}
                    />
                </RuleGroup>

                <RuleGroup
                    title="Featuring"
                    hint="«feat.», «featuring» e simili diventano un unico alias"
                    count={(draft.occurrenciesToReplaceWithFt ?? []).length}
                    tone="blue"
                >
                    <label className="ft-alias">
                        <span>Alias da usare</span>
                        <input
                            type="text"
                            placeholder="ft"
                            value={draft.ftAlias ?? ''}
                            onChange={(e) => onFtAliasChange(e.target.value)}
                            disabled={busy}
                        />
                    </label>
                    <ChipList
                        label="Varianti da sostituire con l'alias"
                        caption="Varianti da sostituire"
                        values={draft.occurrenciesToReplaceWithFt ?? []}
                        onChange={(v) => onListChange('occurrenciesToReplaceWithFt', v)}
                        disabled={busy}
                    />
                </RuleGroup>

                <RuleGroup
                    title="Artisti"
                    hint="Nomi d'arte con « & » o « x » da non dividere nei tag"
                    count={(draft.artistExceptions ?? []).length}
                    tone="green"
                >
                    <ChipList
                        label="Nomi d'arte da non separare"
                        values={draft.artistExceptions ?? []}
                        onChange={(v) => onListChange('artistExceptions', v)}
                        disabled={busy}
                    />
                </RuleGroup>

                <RuleGroup
                    title="File"
                    hint="Estensioni dei file considerati nella cartella"
                    count={(draft.supportedExtensions ?? []).length}
                    tone="gray"
                >
                    <ChipList
                        label="Estensioni supportate"
                        values={draft.supportedExtensions ?? []}
                        onChange={(v) => onListChange('supportedExtensions', v)}
                        disabled={busy}
                    />
                </RuleGroup>
            </div>

            {/* Sostituzioni Da → A: un gruppo per ambito, così
                l'ambito si legge una volta sola nel titolo. */}
            <h3 className="rule-title">Sostituzioni (Da → A)</h3>
            <div className="rule-groups">
                {REPLACEMENT_SCOPES.map((g) => {
                    const rows = (draft.replacements ?? [])
                        .map((r, i) => ({ r, i }))
                        .filter(({ r }) => (r.scope ?? '') === g.scope)
                    return (
                        <RuleGroup key={g.scope} title={g.label} hint={g.hint} count={rows.length} tone="yellow">
                            {rows.length === 0 && <p className="rule-empty">Nessuna sostituzione.</p>}
                            {rows.map(({ r, i }) => (
                                <div className="replacement-row" key={i}>
                                    <input
                                        type="text"
                                        placeholder="Da"
                                        value={r.from}
                                        onChange={(e) => onReplacementChange(i, 'from', e.target.value)}
                                        disabled={busy}
                                    />
                                    <span className="arrow">→</span>
                                    <input
                                        type="text"
                                        placeholder="A"
                                        value={r.to}
                                        onChange={(e) => onReplacementChange(i, 'to', e.target.value)}
                                        disabled={busy}
                                    />
                                    <button
                                        className="ghost small danger"
                                        onClick={() => onRemoveReplacement(i)}
                                        disabled={busy}
                                        aria-label="Rimuovi sostituzione"
                                    >
                                        ✕
                                    </button>
                                </div>
                            ))}
                            <div>
                                <button
                                    className="ghost small add-replacement"
                                    onClick={() => onAddReplacement(g.scope)}
                                    disabled={busy}
                                >
                                    + Aggiungi
                                </button>
                            </div>
                        </RuleGroup>
                    )
                })}
            </div>
        </section>
    )
}
