import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { App as CapApp } from '@capacitor/app'
import './App.css'
import './mobile.css'
import {
    GetState,
    GetConfig,
    SelectFolder,
    SetFolder,
    Scan,
    ProcessAll,
    SetConfig,
    ResetConfig,
    SetAsDefault,
    ClearLogs,
    ChooseDirectory,
    SetOptions,
    SetWatchEnabled,
    OpenFolder,
    ClearTags,
    Cancel,
    SetPlaylists,
    DownloadPlaylist,
    DownloadAndProcess,
    DownloadLink,
    DownloadLinkAndProcess,
    InstallYtDlp,
    InstallFFmpeg,
    UninstallYtDlp,
    UninstallFFmpeg,
    SetYtDlpConfig,
    ChooseYtDlpFile,
    ResolveTagPrompt,
    SuggestTrackNames,
    CheckUpdate,
    MarkUpdateSeen,
    InstallUpdate,
    GoogleSignIn,
    GoogleSignOut,
    GooglePlaylists,
    RefreshGooglePlaylists,
    SyncSettings,
    EmptyGooglePlaylist,
    SetPlaylistPrefs,
    DownloadGooglePlaylist,
    DownloadGooglePlaylistAndProcess,
    AddLinkToPlaylist,
    isAndroid,
    readClipboard,
    onEvent,
    onResume,
    openURL,
    requestNotifications,
    requestStorage,
    storageStatus,
    hasCrashReport,
    shareCrashReport,
    discardCrashReport,
    type core,
    type rules,
    type playlist,
} from './api'
import FolderPicker from './FolderPicker'
import {
    AccountIcon,
    AlertIcon,
    BackIcon,
    ChevronIcon,
    CheckIcon,
    CloseIcon,
    ConvertIcon,
    DownloadIcon,
    EyeIcon,
    FolderOpenIcon,
    InfoCircleIcon,
    PlusIcon,
    RefreshIcon,
    RemoveIcon,
    RulesIcon,
    SettingsIcon,
    TagOffIcon,
} from './icons'
import { ChipList, InfoIcon, CheckOption, Tooltip, Collapse, OpProgress } from './components/controls'
import { ExtChip, CurrentField, ErrorLabel } from './components/files'
import { MissingFolderIcon, FolderLines } from './components/folders'
import { ShortcutsLegend, DefaultsMenu, logKey, ActivityMenu } from './components/HeaderMenus'
import { playlistChoices, PlaylistSelect, PrefsButton, LinkField } from './components/playlists'
import {
    ClearTagsConfirm,
    CrashDialog,
    DeleteOriginalsConfirm,
    DownloadYtDlpConfirm,
    EmptyPlaylistConfirm,
    FFmpegConfirm,
    GoogleSignOutConfirm,
    InstallYtDlpConfirm,
    ResetDefaultsConfirm,
    SaveDefaultsConfirm,
    UninstallFFmpegConfirm,
    UninstallYtDlpConfirm,
} from './dialogs/confirms'
import { DownloadErrorsDialog } from './dialogs/DownloadErrorsDialog'
import { GooglePickerDialog } from './dialogs/GooglePickerDialog'
import { PlaylistPrefsDialog, type PlaylistPrefsTarget } from './dialogs/PlaylistPrefsDialog'
import { TagPromptDialog } from './dialogs/TagPromptDialog'
import { UpdateDialog } from './dialogs/UpdateDialog'
import { usePullToRefresh } from './hooks/usePullToRefresh'
import { cloneConfig, comparableConfig, comparablePlaylists, prefsSummary, REPLACEMENT_SCOPES } from './lib/config'
import { splitName, tagChanged, fileWillChange } from './lib/files'
import { linkInText, playlistIdOf, playlistKeyOf, isVideoLink } from './lib/links'
import { type InstallProgress, installPercent, installLabel } from './lib/progress'
import { type TagPrompt, type PromptSearch, promptsOf } from './lib/prompts'

// Toast: notifica effimera (in basso a destra su desktop, in basso a tutta
// larghezza su Android). È l'unico canale per l'esito delle azioni: `ok`
// decide colore/icona, `duration` (ms) la durata prima della chiusura automatica.
type Toast = { id: number; ok: boolean; message: string; duration: number }

// RuleGroup: card richiudibile di una categoria di regole (scheda Regole). Il
// titolo porta descrizione e numero di voci, così anche da chiusa si capisce
// cosa contiene; tone è il colore della categoria (bordo e contatore). Parte
// chiusa: le card chiuse fanno da indice della scheda.
function RuleGroup({ title, hint, count, tone, children }: {
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

// Repository del progetto, linkato nella scheda Info delle Impostazioni.
const REPO_URL = 'https://github.com/SimoxSpeed/RenameMusic'

// Schede delle Impostazioni, nell'ordine in cui compaiono (su desktop anche
// Ctrl+1…4 e Ctrl+←/→).
type SettingsTab = 'general' | 'download' | 'rules' | 'info'
const SETTINGS_TABS: { id: SettingsTab; label: string; icon: ReactNode }[] = [
    { id: 'general', label: 'Generale', icon: <SettingsIcon /> },
    { id: 'download', label: 'Download', icon: <DownloadIcon /> },
    { id: 'rules', label: 'Regole', icon: <RulesIcon /> },
    { id: 'info', label: 'Info', icon: <InfoCircleIcon size={16} /> },
]

function App() {
    const [state, setState] = useState<core.StateResponse | null>(null)
    const [toasts, setToasts] = useState<Toast[]>([])
    const toastIdRef = useRef(0)
    const [busy, setBusy] = useState(false)
    const [showSettings, setShowSettings] = useState(false)
    // settingsTab: scheda delle Impostazioni aperta; resta quella dell'ultima
    // visita finché l'app è aperta.
    const [settingsTab, setSettingsTab] = useState<SettingsTab>('general')
    // draft / playlistDraft: regole e playlist in editing nelle Impostazioni.
    // Si salvano da sole poco dopo ogni modifica (vedi flushSave); le playlist
    // passano da SetPlaylists perché non fanno parte di rules.Config.
    const [draft, setDraft] = useState<rules.Config | null>(null)
    const [playlistDraft, setPlaylistDraft] = useState<playlist.Playlist[]>([])
    // saveStatus: indicatore del salvataggio automatico nella barra delle
    // Impostazioni ('saved' sparisce da solo dopo poco).
    const [saveStatus, setSaveStatus] = useState<'saving' | 'saved' | null>(null)
    // selectedPlaylist: nome scelto nel select accanto al bottone "Scarica".
    const [selectedPlaylist, setSelectedPlaylist] = useState('')
    // downloadLink: link incollato (di solito un singolo video). Se c'è, il
    // tasto di download scarica lui al posto della playlist selezionata. Su
    // Android si riempie da solo con un link di YouTube appena copiato.
    const [downloadLink, setDownloadLink] = useState('')
    const [results, setResults] = useState<core.ResultView[] | null>(null)
    const [confirmDefault, setConfirmDefault] = useState(false)
    // defaultsMenu: pannello dei predefiniti aperto (solo Android, DefaultsMenu).
    const [defaultsMenu, setDefaultsMenu] = useState(false)
    // Uscendo dalle Impostazioni il pannello si chiude: al rientro è chiuso.
    useEffect(() => {
        if (!showSettings) setDefaultsMenu(false)
    }, [showSettings])
    // activityOpen: pannello del registro Attività aperto (ActivityMenu).
    // logsSeen: chiave dell'ultima riga vista (la più recente all'ultima
    // apertura, o all'avvio): quelle sopra di lei accendono il pallino.
    const [activityOpen, setActivityOpen] = useState(false)
    const [logsSeen, setLogsSeen] = useState('')
    // crashReport: solo Android, l'app si è chiusa in modo anomalo e c'è il
    // registro dell'errore da condividere (popup all'avvio).
    const [crashReport, setCrashReport] = useState(false)
    const [destSameAsSource, setDestSameAsSource] = useState(true)
    const [destFolder, setDestFolder] = useState('')
    const [deleteOriginals, setDeleteOriginals] = useState(false)
    const [watchEnabled, setWatchEnabled] = useState(false)
    // Gestione di yt-dlp: ytDlpManaged rispecchia la checkbox "Gestisci
    // autonomamente"; ytDlpPathDraft è il campo del percorso personalizzato,
    // persistito con SetYtDlpConfig (all'uscita dal campo o via "Sfoglia").
    const [ytDlpManaged, setYtDlpManaged] = useState(true)
    const [ytDlpPathDraft, setYtDlpPathDraft] = useState('')
    // ytDlpChecking: true mentre, riattivata la gestione autonoma, il backend
    // cerca la copia locale di yt-dlp in %AppData% (ed esegue `--version`).
    const [ytDlpChecking, setYtDlpChecking] = useState(false)
    const [confirmDeleteOriginals, setConfirmDeleteOriginals] = useState(false)
    const [confirmClearTags, setConfirmClearTags] = useState(false)
    // confirmInstallYtDlp: popup chiesto quando si preme "Scarica" playlist ma
    // yt-dlp non è presente. Il testo cambia a seconda di ytDlpManaged: se attivo
    // chiede solo il permesso di scaricarlo, altrimenti propone di attivare la
    // gestione automatica e procedere.
    const [confirmInstallYtDlp, setConfirmInstallYtDlp] = useState(false)
    // confirmUninstallYtDlp: popup di conferma per rimuovere la copia gestita.
    const [confirmUninstallYtDlp, setConfirmUninstallYtDlp] = useState(false)
    // confirmUninstallFFmpeg: popup di conferma per rimuovere la copia di
    // ffmpeg gestita dall'app (solo desktop).
    const [confirmUninstallFFmpeg, setConfirmUninstallFFmpeg] = useState(false)
    // confirmDownloadYtDlp: popup di avvertimento prima di scaricare/installare
    // yt-dlp dal tasto dedicato (quando non è presente), separato dal flusso di
    // download di una playlist (confirmInstallYtDlp).
    const [confirmDownloadYtDlp, setConfirmDownloadYtDlp] = useState(false)
    // confirmFFmpeg: popup prima di scaricare ffmpeg (solo desktop; serve a
    // yt-dlp per creare gli mp3). 'install' dal tasto nel pannello, 'playlist'
    // quando si preme "Scarica" di una playlist e ffmpeg manca (poi prosegue).
    const [confirmFFmpeg, setConfirmFFmpeg] = useState<null | 'install' | 'playlist'>(null)
    // confirmReset: popup prima di "Ripristina predefiniti", che sovrascrive
    // subito le regole e le playlist correnti.
    const [confirmReset, setConfirmReset] = useState(false)
    // progress: avanzamento dell'ultima elaborazione (x/totale), popolato dagli
    // eventi process:progress durante ProcessAll; null quando non pertinente.
    // phase ('download' | 'convert') distingue le due fasi di "Scarica e converti".
    const [progress, setProgress] = useState<{ done: number; total: number; phase?: string } | null>(null)
    // installProgress: avanzamento del download di yt-dlp/ffmpeg (evento
    // install:progress); azzerato quando l'installazione risponde.
    const [installProgress, setInstallProgress] = useState<InstallProgress | null>(null)
    // showOnlyChanged: vista dell'anteprima limitata ai soli file che cambieranno
    // nome. È SOLO una vista: l'elaborazione tratta comunque tutti i file.
    const [showOnlyChanged, setShowOnlyChanged] = useState(false)
    // reviewPaths: file spuntati nell'anteprima da rivedere. Alla conversione
    // non vengono convertiti subito ma passano dal popup, come le tracce non
    // rinominabili.
    const [reviewPaths, setReviewPaths] = useState<Set<string>>(() => new Set())
    // cancellable: true mentre è in corso un'operazione interrompibile (ProcessAll
    // o ClearTags), così mostriamo il tasto "Annulla".
    const [cancellable, setCancellable] = useState(false)
    // booted diventa true al termine del caricamento iniziale: finché è false
    // mostriamo un placeholder di caricamento invece del messaggio "vuoto",
    // così al refresh non si vede un lampo di stato vuoto prima dei dati.
    const [booted, setBooted] = useState(false)
    // tagPrompts: coda delle tracce con tag sconosciuti segnalate durante
    // ProcessAll (evento process:needTagInput). Mostriamo un popup per la prima
    // della coda; risolverla (Salta/Continua) la rimuove e scopre la successiva.
    // La conversione delle altre tracce prosegue in background nel frattempo.
    const [tagPrompts, setTagPrompts] = useState<TagPrompt[]>([])
    // promptDraft: nome modificabile nell'input del popup, inizializzato dal nome
    // originale della traccia in testa alla coda.
    const [promptDraft, setPromptDraft] = useState('')
    // promptSearch: ricerca su MusicBrainz della traccia in testa alla coda
    // (SuggestTrackNames): finché è in corso il popup mostra solo l'attesa,
    // poi il campo con i nomi trovati (o l'errore). null = nessuna ricerca
    // (disattivata). suggestionsRef: una ricerca per traccia, condivisa fra la
    // ricerca in anticipo e il popup; stopSearchRef interrompe l'attesa
    // ("Salta ricerca": il risultato che arriva dopo si ignora).
    const [promptSearch, setPromptSearch] = useState<PromptSearch | null>(null)
    const suggestionsRef = useRef(new Map<string, Promise<core.TrackSuggestions>>())
    const stopSearchRef = useRef<(() => void) | null>(null)
    // promptFilter: testo cercato fra i brani trovati su MusicBrainz.
    const [promptFilter, setPromptFilter] = useState('')
    // downloadErrors: video di playlist non scaricati nell'ultimo download (con
    // dettaglio dell'errore). Popolato dalla risposta di DownloadPlaylist; un
    // badge nell'area download apre il modale che li elenca (showDownloadErrors).
    const [downloadErrors, setDownloadErrors] = useState<core.DownloadErrorView[]>([])
    const [showDownloadErrors, setShowDownloadErrors] = useState(false)
    // Solo Android. storageGranted: accesso a tutti i file concesso (sempre true
    // su desktop); senza, il core non può leggere né rinominare i file e la UI
    // mostra la richiesta di permesso. folderPicker: selettore cartelle interno
    // (su Android sostituisce il dialog di sistema) aperto per la cartella di
    // partenza ('source') o di destinazione ('dest').
    const [storageGranted, setStorageGranted] = useState(true)
    const [folderPicker, setFolderPicker] = useState<'source' | 'dest' | null>(null)
    // updatePopup: nuova versione dell'app mostrata nel popup di aggiornamento.
    // Si apre da solo una volta per versione (poi resta il tasto "Aggiorna"
    // nelle Impostazioni); updateShownRef ricorda la versione già mostrata in
    // questa sessione, per non riaprirlo prima che il core la segni come vista.
    const [updatePopup, setUpdatePopup] = useState<core.UpdateView | null>(null)
    const updateShownRef = useRef('')
    // Account Google (playlist di YouTube). googleSigningIn: accesso in corso
    // (su desktop si completa nel browser e si può annullare). googlePicker:
    // popup con le playlist dell'account, per importarle nelle Impostazioni
    // ('import') o per aggiungerci il video del link ('add'); googleLists è
    // null finché l'elenco non arriva, googleImport sono gli ID spuntati per
    // l'importazione. pickerReqRef scarta le risposte di un popup già chiuso.
    const [googleSigningIn, setGoogleSigningIn] = useState(false)
    const [confirmGoogleSignOut, setConfirmGoogleSignOut] = useState(false)
    const [googlePicker, setGooglePicker] = useState<null | 'import' | 'add'>(null)
    const [googleLists, setGoogleLists] = useState<core.GooglePlaylistView[] | null>(null)
    const [googleImport, setGoogleImport] = useState<Set<string>>(() => new Set())
    const pickerReqRef = useRef(0)
    // Impostazioni di una playlist (Impostazioni > Download, tasto ⚙):
    // prefsTarget è la playlist aperta (key come StateResponse.playlistPrefs;
    // ownId è il suo ID se è dell'account, quindi si può svuotare), prefsDraft
    // le impostazioni in modifica. confirmEmpty chiede conferma prima di
    // «Svuota ora»; emptying è l'ID della playlist che si sta svuotando.
    const [prefsTarget, setPrefsTarget] = useState<PlaylistPrefsTarget | null>(null)
    const [prefsDraft, setPrefsDraft] = useState<playlist.Prefs>({})
    const [confirmEmpty, setConfirmEmpty] = useState<{ id: string; title: string } | null>(null)
    const [emptying, setEmptying] = useState('')
    // prefsSyncing: controllo in corso delle impostazioni sull'account Google.
    const [prefsSyncing, setPrefsSyncing] = useState(false)

    // showSettingsRef rispecchia showSettings per absorbState, che gira anche
    // nei gestori di eventi registrati una sola volta (vedi folder:dropped) e al
    // ritorno di chiamate partite prima di un cambio di schermata.
    const showSettingsRef = useRef(false)
    showSettingsRef.current = showSettings
    // Copie sempre aggiornate di stato e bozze per il salvataggio automatico,
    // che parte da un timer e deve vedere i valori più recenti.
    const stateRef = useRef(state)
    stateRef.current = state
    const draftRef = useRef(draft)
    draftRef.current = draft
    const playlistDraftRef = useRef(playlistDraft)
    playlistDraftRef.current = playlistDraft
    // Contenitore che scorre sotto l'header (vedi .app-scroll in App.css).
    const scrollRef = useRef<HTMLDivElement>(null)

    // downloadRowWidth: larghezza della riga playlist + "Scarica" della
    // schermata normale su desktop, misurata con la playlist aperta e ferma. Il
    // campo del link non la supera e, con la playlist chiusa, "Scarica" si
    // allarga fino a lei (--download-row-width in App.css). Su Android la riga
    // occupa già tutta la larghezza.
    const downloadRowRef = useRef<HTMLDivElement>(null)
    const [downloadRowWidth, setDownloadRowWidth] = useState(0)
    function measureDownloadRow() {
        const row = downloadRowRef.current
        if (isAndroid || !row || row.querySelector(':scope > [data-away]')) return
        const last = row.lastElementChild
        if (!last) return
        const width = Math.ceil(last.getBoundingClientRect().right - row.getBoundingClientRect().left)
        setDownloadRowWidth((prev) => (prev === width ? prev : width))
    }

    // A ogni render (playlist scelta, errori di download, ...) e quando i font
    // sono pronti, che cambiano la larghezza del testo.
    useLayoutEffect(measureDownloadRow)
    useEffect(() => {
        document.fonts?.ready.then(measureDownloadRow)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    function absorb(resp: core.ActionResponse, resetDrafts = false) {
        absorbState(resp.state, resetDrafts)
    }

    // absorbState riporta nella UI lo stato del core. Fuori dalle Impostazioni
    // riallinea anche le bozze (regole e playlist in editing) allo stato
    // salvato. Nelle Impostazioni invece le lascia com'erano: il core normalizza
    // ciò che salva (scarta righe vuote, pulisce le liste) e riallineare a ogni
    // salvataggio automatico toglierebbe di mano la riga appena aggiunta o lo
    // spazio appena digitato. Allinea solo la cartella della bozza, che il core
    // gestisce a parte. resetDrafts forza il riallineamento: serve al ripristino
    // dei predefiniti, che cambia proprio lo stato salvato delle bozze.
    function absorbState(next: core.StateResponse, resetDrafts = false) {
        noteSyncRevision(next)
        setState(next)
        stateRef.current = next
        syncSelectedPlaylist(next)
        if (showSettingsRef.current && !resetDrafts) {
            setDraft((prev) => (prev ? ({ ...prev, startFolder: next.folder } as rules.Config) : prev))
            return
        }
        syncDrafts(next)
    }

    // La playlist scelta resta quella, se c'è ancora; altrimenti la prima.
    function syncSelectedPlaylist(s: core.StateResponse) {
        const choices = playlistChoices(s.google?.playlists ?? [], s.playlists ?? [], s.playlistPrefs ?? {}).filter((c) => !c.separator)
        setSelectedPlaylist((prev) => (choices.some((c) => c.value === prev) ? prev : (choices[0]?.value ?? '')))
    }

    // Stato dell'account Google aggiornato fuori dal resto (lettura delle
    // playlist, sincronizzazione delle impostazioni con l'account). Di solito
    // cambiano solo account e impostazioni delle playlist, e si tocca solo
    // quello; se dall'account sono arrivate regole o playlist di un altro
    // dispositivo si assorbe tutto lo stato, e nelle Impostazioni si
    // riallineano anche le bozze, purché non siano state modificate nel
    // frattempo (altrimenti vince la modifica appena fatta).
    function absorbSynced(next: core.StateResponse) {
        noteSyncRevision(next)
        const prev = stateRef.current
        const rulesChanged =
            !!prev?.config &&
            !!next.config &&
            (comparableConfig(prev.config) !== comparableConfig(next.config) ||
                comparablePlaylists(prev.playlists ?? []) !== comparablePlaylists(next.playlists ?? []))
        if (!prev || !rulesChanged) {
            const patch = { google: next.google, playlistPrefs: next.playlistPrefs, logs: next.logs }
            setState((p) => (p ? ({ ...p, ...patch } as core.StateResponse) : p))
            if (prev) syncSelectedPlaylist({ ...prev, ...patch } as core.StateResponse)
            return
        }
        const draftsClean =
            !!draftRef.current &&
            comparableConfig(draftRef.current) === comparableConfig(prev.config) &&
            comparablePlaylists(playlistDraftRef.current) === comparablePlaylists(prev.playlists ?? [])
        absorbState(next)
        if (showSettingsRef.current && draftsClean) syncDrafts(next)
    }

    // Avviso quando arrivano impostazioni cambiate su un altro dispositivo
    // (GoogleView.syncRevision cresce). syncRevisionRef è -1 finché non si è
    // visto il primo stato: all'avvio non si avvisa.
    const syncRevisionRef = useRef(-1)
    function noteSyncRevision(next: core.StateResponse) {
        const rev = next.google?.syncRevision ?? 0
        if (syncRevisionRef.current >= 0 && rev > syncRevisionRef.current) {
            notify(true, 'Impostazioni aggiornate da un altro dispositivo.')
        }
        syncRevisionRef.current = Math.max(syncRevisionRef.current, rev)
    }

    function syncDrafts(s: core.StateResponse) {
        if (s.config) setDraft(cloneConfig(s.config))
        setPlaylistDraft(
            (s.playlists ?? []).map((p) => ({ name: p.name, url: p.url })),
        )
    }

    // Durata minima (ms) per cui lo stato "busy" resta attivo una volta partito:
    // così la barra di progresso non lampeggia (accendendosi e spegnendosi in
    // pochi ms) sulle operazioni rapide, ma resta visibile un istante coerente.
    const MIN_BUSY_MS = 450

    async function guard(fn: () => Promise<void>) {
        const start = performance.now()
        setBusy(true)
        try {
            await fn()
        } catch (err: any) {
            notify(false, 'Errore: ' + (err?.message ?? String(err)))
        } finally {
            const elapsed = performance.now() - start
            if (elapsed < MIN_BUSY_MS) {
                await new Promise((r) => window.setTimeout(r, MIN_BUSY_MS - elapsed))
            }
            setBusy(false)
        }
    }

    // Numero massimo di toast impilati: oltre, il più vecchio lascia il posto.
    const MAX_TOASTS = 3

    // notify mostra un toast effimero. Gli errori restano più a lungo (portano
    // un messaggio da leggere); i successi spariscono in fretta. Messaggio vuoto
    // => nessun toast. Un messaggio identico a uno già visibile lo sostituisce
    // (riportandolo in fondo e facendo ripartire il conto alla rovescia) invece
    // di impilarsi. La chiusura automatica non usa un timer JS: la decide la
    // fine dell'animazione della barra del toast (vedi render), così barra e
    // scadenza coincidono anche quando la barra è in pausa (hover su desktop).
    function notify(ok: boolean, message: string) {
        if (!message) return
        const id = (toastIdRef.current += 1)
        const duration = ok ? 2500 : 3500
        setToasts((prev) =>
            [...prev.filter((t) => t.ok !== ok || t.message !== message), { id, ok, message, duration }].slice(
                -MAX_TOASTS,
            ),
        )
    }

    function dismissToast(id: number) {
        setToasts((prev) => prev.filter((t) => t.id !== id))
    }

    function syncOptions(s: core.StateResponse) {
        setDestSameAsSource(s.destinationSameAsSource)
        setDestFolder(s.destinationFolder ?? '')
        setDeleteOriginals(s.deleteOriginals)
        setWatchEnabled(s.watchEnabled)
        setYtDlpManaged(s.ytDlpManaged)
        setYtDlpPathDraft(s.ytDlpPath ?? '')
    }

    // Carica lo stato iniziale (cartella + opzioni + anteprima) in UN SOLO passaggio:
    // il backend (GetState) scansiona già la cartella ricordata e restituisce le
    // anteprime, quindi la UI si popola una sola volta senza svuotarsi/riempirsi.
    // Il ref evita la doppia esecuzione indotta da React.StrictMode in dev.
    const bootedRef = useRef(false)
    useEffect(() => {
        if (bootedRef.current) return
        bootedRef.current = true
        guard(async () => {
            // Prima lo stato senza scansione (GetConfig, immediato): serve a
            // sapere subito se la modalità semplificata è attiva e disegnare
            // da subito la schermata giusta, invece di mostrare quella normale
            // finché la scansione di GetState non è finita.
            const quick = await GetConfig()
            absorb(quick)
            syncOptions(quick.state)
            // Le righe già nel registro all'avvio non accendono il pallino di
            // Attività (né quelle della scansione iniziale, qui sotto).
            setLogsSeen(logKey(quick.state.logs?.[0]))
            // Android: senza accesso ai file la scansione vedrebbe una cartella
            // vuota; controlliamo prima il permesso (la UI lo chiede se manca).
            if (isAndroid) {
                const granted = await storageStatus()
                storageRef.current = granted
                setStorageGranted(granted)
                if (granted) requestNotifications()
            }
            const resp = await GetState()
            absorb(resp)
            syncOptions(resp.state)
            setLogsSeen(logKey(resp.state.logs?.[0]))
            // Niente toast all'avvio: l'anteprima popolata basta a dire che la
            // scansione è andata, e un eventuale errore finisce già in Attività.
        }).finally(() => setBooted(true))
        hasCrashReport()
            .then(setCrashReport)
            .catch(() => {})
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Android: al ritorno in primo piano ricontrolliamo il permesso sui file
    // (l'utente lo concede nelle impostazioni di sistema, fuori dall'app). Se è
    // appena stato concesso e c'è già una cartella ricordata la riscansioniamo,
    // così l'anteprima si popola subito. I ref servono perché il gestore è
    // registrato una sola volta e non vedrebbe lo stato aggiornato.
    const storageRef = useRef(true)
    const folderRef = useRef('')
    useEffect(() => {
        return onResume((granted) => {
            const wasGranted = storageRef.current
            storageRef.current = granted
            setStorageGranted(granted)
            if (granted && !wasGranted) {
                requestNotifications()
                if (folderRef.current) refresh()
            } else {
                recheckFolders()
            }
            refreshGoogleLists()
        })
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Android: "Incolla" nel campo del link legge gli appunti (solo con quel
    // tocco) e ci mette il link che contengono, pronto da scaricare.
    function pasteLink() {
        readClipboard()
            .then((text) => {
                const url = linkInText(text)
                if (url) setDownloadLink(url)
                else notify(false, text.trim() ? 'Negli appunti non c\'è un link.' : 'Gli appunti sono vuoti.')
            })
            .catch((err: any) => notify(false, 'Impossibile leggere gli appunti: ' + (err?.message ?? String(err))))
    }

    // Chiede l'accesso a tutti i file (Android 11+: apre le impostazioni di
    // sistema; lo stato si aggiorna al ritorno nell'app, vedi onResume).
    function askStorage() {
        requestStorage()
            .then((granted) => {
                if (granted && !storageRef.current) {
                    storageRef.current = true
                    setStorageGranted(true)
                    if (folderRef.current) refresh()
                }
            })
            .catch((e) => notify(false, 'Impossibile richiedere il permesso: ' + String(e)))
    }

    // Rilegge le playlist dell'account Google, se collegato. Un accesso
    // scaduto scollega l'account e lo dice con un toast.
    function refreshGoogleLists() {
        if (!stateRef.current?.google?.connected) return
        RefreshGooglePlaylists()
            .then((resp) => {
                absorbSynced(resp.state)
                if (!resp.ok) notify(false, resp.message ?? '')
            })
            .catch(() => {})
    }

    // Playlist dell'account lette dal core all'avvio (o accesso scaduto).
    useEffect(() => {
        return onEvent('google:changed', (payload: unknown) => {
            const next = payload as core.StateResponse
            if (next) absorbSynced(next)
        })
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Riporta nello stato solo se le cartelle impostate si trovano ancora sul disco.
    function recheckFolders() {
        GetConfig()
            .then((resp) =>
                setState((prev) =>
                    prev
                        ? ({
                              ...prev,
                              folderMissing: resp.state.folderMissing,
                              destinationMissing: resp.state.destinationMissing,
                          } as core.StateResponse)
                        : prev,
                ),
            )
            .catch(() => {})
    }

    // Aggiorna e persiste le opzioni di elaborazione (destinazione + eliminazione originali).
    function applyOptions(same: boolean, dest: string, del: boolean) {
        setDestSameAsSource(same)
        setDestFolder(dest)
        setDeleteOriginals(del)
        SetOptions(same, dest, del)
            .then((resp) =>
                setState((prev) =>
                    prev ? ({ ...prev, destinationMissing: resp.state.destinationMissing } as core.StateResponse) : prev,
                ),
            )
            .catch(() => {
                /* la persistenza opzioni non deve bloccare la UI */
            })
    }

    // In modalità watch, il backend rileva variazioni nella cartella e ci
    // manda lo stato aggiornato: aggiorniamo solo l'anteprima (la conversione
    // resta manuale). Se l'utente sta guardando i risultati dell'ultima
    // conversione, ignoriamo l'evento: il backend è già in pausa in quel caso,
    // ma è una difesa extra lato UI.
    useEffect(() => {
        return onEvent('watch:changed', (payload: unknown) => {
            const next = payload as core.StateResponse
            if (!next) return
            setState((prev) =>
                prev
                    ? ({
                          ...prev,
                          files: next.files,
                          logs: next.logs,
                          folderMissing: next.folderMissing,
                          destinationMissing: next.destinationMissing,
                      } as core.StateResponse)
                    : next,
            )
        })
    }, [])

    // Stato di yt-dlp cambiato fuori da una richiesta della UI (Android: fine
    // dell'inizializzazione di youtubedl-android all'avvio): aggiorniamo solo
    // presenza/versione, senza toccare il resto dello stato.
    useEffect(() => {
        return onEvent('ytdlp:changed', (payload: unknown) => {
            const next = payload as core.StateResponse
            if (!next) return
            setState((prev) =>
                prev
                    ? ({
                          ...prev,
                          ytDlpAvailable: next.ytDlpAvailable,
                          ytDlpVersion: next.ytDlpVersion,
                          ytDlpEffectivePath: next.ytDlpEffectivePath,
                          ffmpegAvailable: next.ffmpegAvailable,
                          logs: next.logs,
                      } as core.StateResponse)
                    : next,
            )
        })
    }, [])

    // Trascinamento di una cartella sulla finestra: il backend imposta la
    // cartella di partenza e ci manda lo stato aggiornato (con l'anteprima).
    // Assorbiamo tutto come farebbe una scansione manuale.
    useEffect(() => {
        return onEvent('folder:dropped', (payload: unknown) => {
            const next = payload as core.StateResponse
            if (!next) return
            absorbState(next)
            setResults(null)
            syncOptions(next)
            const ok = next.folder !== ''
            const msg = ok ? 'Cartella impostata dal trascinamento.' : 'Trascinamento non valido.'
            notify(ok, msg)
        })
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Avanzamento di ProcessAll: il backend emette un evento per ogni file
    // completato; aggiorniamo il contatore della barra di avanzamento.
    useEffect(() => {
        return onEvent('process:progress', (payload: unknown) => {
            const p = payload as { done: number; total: number; phase?: string } | null
            if (p) setProgress(p)
        })
    }, [])

    // Avanzamento del download di yt-dlp/ffmpeg: il backend emette un evento a
    // ogni punto percentuale in più.
    useEffect(() => {
        return onEvent('install:progress', (payload: unknown) => {
            const p = payload as InstallProgress | null
            if (p) setInstallProgress(p)
        })
    }, [])

    // absorbUpdate riporta nello stato solo l'aggiornamento disponibile e il
    // registro attività: le risposte dei metodi di aggiornamento non devono
    // passare da absorb, che reimposterebbe le bozze delle Impostazioni.
    function absorbUpdate(next: core.StateResponse) {
        setState((prev) => (prev ? ({ ...prev, update: next.update, logs: next.logs } as core.StateResponse) : prev))
    }

    // Nuova versione trovata dal controllo periodico del core.
    useEffect(() => {
        return onEvent('update:available', (payload: unknown) => {
            const next = payload as core.UpdateView | null
            if (!next) return
            setState((prev) => (prev ? ({ ...prev, update: next } as core.StateResponse) : prev))
        })
    }, [])

    // Al ritorno della connessione controlliamo subito gli aggiornamenti, senza
    // aspettare il prossimo tentativo periodico del core. Silenzioso: l'esito
    // arriva solo come popup, se c'è una nuova versione.
    useEffect(() => {
        const onOnline = () => {
            CheckUpdate()
                .then((resp) => absorbUpdate(resp.state))
                .catch(() => {})
        }
        window.addEventListener('online', onOnline)
        return () => window.removeEventListener('online', onOnline)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Al ritorno sulla finestra ricontrolliamo che le cartelle esistano ancora
    // (nel frattempo possono essere state spostate o eliminate): aggiorniamo solo
    // gli avvisi, senza toccare anteprima e bozze. GetConfig non scansiona.
    // Anche le playlist dell'account Google si rileggono (il core lo fa solo
    // se l'ultima lettura non è recente): così compaiono quelle appena create.
    useEffect(() => {
        const onFocus = () => {
            recheckFolders()
            refreshGoogleLists()
        }
        window.addEventListener('focus', onFocus)
        return () => window.removeEventListener('focus', onFocus)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // Popup "nuova versione" mostrato da solo una volta per versione: appena il
    // core ne segnala una non ancora vista la segniamo come vista (persistito),
    // così non ricompare ai prossimi avvii. Aspetta che non ci siano operazioni
    // in corso né altre scelte in sospeso, per non coprirle.
    const availableUpdate = state?.update
    const updateBlocked = busy || tagPrompts.length > 0 || folderPicker !== null
    useEffect(() => {
        if (!availableUpdate || availableUpdate.seen || updateBlocked) return
        if (updateShownRef.current === availableUpdate.version) return
        updateShownRef.current = availableUpdate.version
        setUpdatePopup(availableUpdate)
        MarkUpdateSeen(availableUpdate.version)
            .then((resp) => absorbUpdate(resp.state))
            .catch(() => {})
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [availableUpdate, updateBlocked])

    // Quando cambia la traccia in testa alla coda, reimpostiamo l'input del popup
    // al suo nome originale (l'utente riparte dal nome da correggere) o, per una
    // traccia selezionata da rivedere, al nome proposto dall'anteprima. Se è
    // attiva, prima del campo c'è la ricerca su MusicBrainz: il primo nome
    // trovato va nell'input, gli altri restano da scegliere. Finita questa,
    // parte quella in anticipo per la traccia successiva (MusicBrainz accetta
    // una richiesta al secondo: così non rallenta quella che si sta aspettando).
    const headPromptPath = tagPrompts[0]?.path
    const musicBrainzOn = !!state?.config?.musicBrainz
    useEffect(() => {
        const head = tagPrompts[0]
        setPromptDraft(head ? (head.review ? head.previewBase : head.originalBase) : '')
        setPromptFilter('')
        if (!head || !musicBrainzOn) {
            setPromptSearch(null)
            return
        }
        setPromptSearch({ loading: true, names: [], error: '' })
        let current = true
        stopSearchRef.current = () => {
            current = false
        }
        const next = tagPrompts[1]
        suggestFor(head.path).then((res) => {
            if (next) void suggestFor(next.path)
            if (!current) return
            const names = res.names ?? []
            if (names.length > 0) setPromptDraft(names[0])
            setPromptSearch({ loading: false, names, error: res.error ?? '' })
        })
        return () => {
            current = false
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [headPromptPath])

    // suggestFor avvia (una volta sola per traccia) la ricerca su MusicBrainz.
    function suggestFor(path: string): Promise<core.TrackSuggestions> {
        let pending = suggestionsRef.current.get(path)
        if (!pending) {
            pending = SuggestTrackNames(path).catch(
                (err) => ({ names: [], error: err?.message ?? String(err) }) as core.TrackSuggestions,
            )
            suggestionsRef.current.set(path, pending)
        }
        return pending
    }

    // "Salta ricerca": il popup mostra subito il campo, col nome di prima.
    function skipPromptSearch() {
        stopSearchRef.current?.()
        setPromptSearch({ loading: false, names: [], error: '', skipped: true })
    }

    // Attiva/disattiva l'aggiornamento automatico. È un semplice cambio di
    // impostazione (avvia/ferma il watcher, nessuna scansione): niente busy a
    // tutta UI, che farebbe sembrare il toggle lento. Flippiamo subito in modo
    // ottimistico e persistiamo in background, assorbendo lo stato reale al
    // ritorno: se il backend non riesce ad avviare il watcher rimette
    // watchEnabled a false e ci allineiamo. Come toggleYtDlpManaged.
    function toggleWatch(next: boolean) {
        setWatchEnabled(next)
        SetWatchEnabled(next)
            .then((resp) => {
                absorb(resp)
                setWatchEnabled(resp.state.watchEnabled)
                notify(resp.ok, resp.message ?? '')
            })
            .catch((e) => {
                setWatchEnabled(!next)
                notify(false, String(e))
            })
    }

    // Riscansiona la cartella corrente (utile se il contenuto è cambiato).
    function refresh() {
        return guard(async () => {
            const resp = await Scan()
            absorb(resp)
            setResults(null)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Scegli cartella: imposta il percorso e mostra subito l'anteprima (scan automatico).
    // Su Android apre il selettore interno (vedi pickFolder).
    function chooseFolder() {
        if (isAndroid) {
            setFolderPicker('source')
            return
        }
        guard(async () => {
            const selected = await SelectFolder()
            if (!selected.ok) {
                absorb(selected)
                setResults(null)
                notify(selected.ok, selected.message ?? '')
                return
            }
            const scanned = await Scan()
            absorb(scanned)
            setResults(null)
            notify(scanned.ok, scanned.message ?? '')
        })
    }

    // Esito del selettore cartelle interno (Android): stessa sequenza del dialog
    // desktop, ma con il percorso già scelto dall'utente nella UI.
    function pickFolder(path: string) {
        const target = folderPicker
        setFolderPicker(null)
        if (target === 'dest') {
            applyOptions(destSameAsSource, path, deleteOriginals)
            notify(true, 'Cartella di destinazione impostata.')
            return
        }
        guard(async () => {
            const selected = await SetFolder(path)
            if (!selected.ok) {
                absorb(selected)
                setResults(null)
                notify(selected.ok, selected.message ?? '')
                return
            }
            const scanned = await Scan()
            absorb(scanned)
            setResults(null)
            notify(scanned.ok, scanned.message ?? '')
        })
    }

    function chooseDestination() {
        if (isAndroid) {
            setFolderPicker('dest')
            return
        }
        guard(async () => {
            const path = await ChooseDirectory()
            if (path) {
                applyOptions(destSameAsSource, path, deleteOriginals)
                notify(true, 'Cartella di destinazione impostata.')
            } else {
                // Selezione annullata: manteniamo la destinazione precedente.
                // ok:false → toast rosso, coerente con l'annullamento della
                // cartella di partenza.
                notify(false, 'Selezione annullata.')
            }
        })
    }

    // Apre una cartella nel file manager di sistema (Esplora risorse). Non è
    // un'operazione bloccante: non usiamo `guard` per non accendere la barra.
    // In caso di errore mostriamo un toast (utile anche a diagnosticare: se il
    // binding non fosse disponibile, la Promise verrebbe rifiutata).
    function openFolder(path: string) {
        if (!path) return
        Promise.resolve(OpenFolder(path))
            .then((resp) => {
                if (resp && !resp.ok) notify(false, resp.message || 'Impossibile aprire la cartella.')
            })
            .catch((err) => notify(false, 'Impossibile aprire la cartella: ' + (err?.message ?? String(err))))
    }

    // Processo unificato: normalizzazione nomi + scrittura tag in un colpo solo.
    // Usa le opzioni persistite lato backend.
    function process() {
        if (!destSameAsSource && destFolder === '') {
            notify(false, 'Scegli una cartella di destinazione o riattiva "uguale alla partenza".')
            return
        }
        setProgress(null)
        setTagPrompts([])
        setCancellable(true)
        guard(async () => {
            const resp = await ProcessAll([...reviewPaths])
            setReviewPaths(new Set())
            // Operazione conclusa: non è più annullabile (evita che un click sul
            // tasto Annulla durante la coda "busy" lasci un annullamento appeso).
            setCancellable(false)
            absorb(resp)
            setResults(resp.results ?? [])
            notify(resp.ok, resp.message ?? '')
            // Tracce con tag sconosciuti o selezionate da rivedere: il backend NON le ha convertite, le
            // rimette qui perché l'utente decida (una alla volta) col popup. Le
            // tracce a posto sono già state convertite: niente blocca.
            setTagPrompts(promptsOf(resp))
        }).finally(() => {
            setProgress(null)
            setCancellable(false)
        })
    }

    // Risolve il popup in testa alla coda: "Salta" (useEdited=false) converte la
    // traccia col nome originale; "Continua" (useEdited=true) usa il nome
    // modificato (da cui il backend riestrae i tag). La conversione della singola
    // traccia è una chiamata a sé (non bloccante): il suo esito viene aggiunto
    // alla tabella dei risultati e la coda avanza alla traccia successiva.
    function resolvePrompt(useEdited: boolean) {
        const head = tagPrompts[0]
        if (!head) return
        const edited = useEdited ? promptDraft : ''
        suggestionsRef.current.delete(head.path)
        // Rimuoviamo subito il popup dalla coda (mostra l'eventuale successivo);
        // la conversione prosegue in background e ne aggiungiamo l'esito.
        setTagPrompts((prev) => prev.slice(1))
        ResolveTagPrompt(head.path, useEdited, edited)
            .then((resp) => {
                setState((prev) => (prev ? ({ ...prev, logs: resp.state.logs } as core.StateResponse) : resp.state))
                const added = resp.results ?? []
                if (added.length > 0) {
                    setResults((prev) => [...(prev ?? []), ...added])
                }
            })
            .catch((err) => notify(false, 'Errore sulla traccia: ' + (err?.message ?? String(err))))
    }

    // Cancella TUTTI i tag ID3 dagli MP3 della cartella (azione distruttiva:
    // confermata da un popup). Non rinomina nulla, agisce in posto.
    function confirmClearTagsAction() {
        setConfirmClearTags(false)
        setProgress(null)
        setCancellable(true)
        guard(async () => {
            const resp = await ClearTags()
            setCancellable(false)
            absorb(resp)
            setResults(null)
            notify(resp.ok, resp.message ?? '')
        }).finally(() => {
            setProgress(null)
            setCancellable(false)
        })
    }

    // Richiede al backend di interrompere l'operazione in corso (conversione o
    // cancellazione tag). Il backend si ferma tra un file e l'altro; la Promise
    // dell'operazione si risolve poi con l'esito parziale.
    function cancelOp() {
        // Azzeriamo subito il contatore: altrimenti la barra "x / totale"
        // resterebbe congelata finché l'operazione non ritorna.
        setProgress(null)
        notify(false, 'Annullamento in corso…')
        Cancel().catch(() => {
            /* l'annullamento non deve generare errori bloccanti in UI */
        })
    }

    // Salvataggio automatico delle Impostazioni: ogni modifica alle bozze si
    // salva da sola AUTOSAVE_MS dopo l'ultima, e comunque prima di uscire o di
    // toccare i predefiniti (flushSave). Non passa da guard: busy disabiliterebbe
    // i campi, e quello in cui si sta scrivendo perderebbe il focus. I
    // salvataggi sono in fila (saveChainRef) e ognuno legge le bozze del momento
    // in cui parte: vince l'ultimo.
    const AUTOSAVE_MS = 600
    const saveTimerRef = useRef(0)
    const savedTimerRef = useRef(0)
    const saveChainRef = useRef<Promise<void>>(Promise.resolve())

    useEffect(() => {
        if (!showSettingsRef.current) return
        window.clearTimeout(saveTimerRef.current)
        saveTimerRef.current = window.setTimeout(flushSave, AUTOSAVE_MS)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [draft, playlistDraft])

    function flushSave(): Promise<void> {
        window.clearTimeout(saveTimerRef.current)
        saveChainRef.current = saveChainRef.current.then(saveDrafts)
        return saveChainRef.current
    }

    // saveDrafts salva solo ciò che differisce dallo stato salvato: le regole
    // con SetConfig (che riscansiona la cartella), le playlist con SetPlaylists.
    // Non rifiuta mai, così la fila dei salvataggi non si interrompe.
    async function saveDrafts() {
        const cfg = draftRef.current
        const pl = playlistDraftRef.current
        const saved = stateRef.current
        if (!cfg || !saved?.config) return
        const cfgChanged = comparableConfig(cfg) !== comparableConfig(saved.config)
        const plChanged = comparablePlaylists(pl) !== comparablePlaylists(saved.playlists ?? [])
        if (!cfgChanged && !plChanged) return

        window.clearTimeout(savedTimerRef.current)
        setSaveStatus('saving')
        let ok = true
        try {
            if (cfgChanged) {
                const resp = await SetConfig(cfg)
                absorb(resp)
                setResults(null)
                ok = resp.ok
                if (!resp.ok) notify(false, resp.message ?? '')
            }
            if (plChanged) {
                const resp = await SetPlaylists(pl)
                absorb(resp)
                if (!resp.ok) notify(false, resp.message ?? '')
                ok = ok && resp.ok
            }
        } catch (err: any) {
            ok = false
            notify(false, 'Errore: ' + (err?.message ?? String(err)))
        }
        setSaveStatus(ok ? 'saved' : null)
        if (ok) savedTimerRef.current = window.setTimeout(() => setSaveStatus(null), 1800)
    }

    // openSettings apre le Impostazioni, sulla scheda indicata o sull'ultima usata.
    function openSettings(tab?: SettingsTab) {
        if (tab) setSettingsTab(tab)
        setShowSettings(true)
        syncPrefsFromAccount()
        refreshGoogleLists()
    }

    // All'apertura delle Impostazioni si ricontrollano subito le impostazioni
    // delle playlist salvate sull'account (cambiate da un altro dispositivo),
    // senza busy: intanto il pannello dell'account mostra il controllo in corso.
    function syncPrefsFromAccount() {
        if (!stateRef.current?.google?.connected) return
        setPrefsSyncing(true)
        SyncSettings()
            .then((resp) => absorbSynced(resp.state))
            .catch(() => {})
            .finally(() => setPrefsSyncing(false))
    }

    // Pull to refresh nelle Impostazioni: rilegge dall'account Google le
    // impostazioni condivise e le playlist, queste ultime anche se lette da
    // poco (a differenza di RefreshGooglePlaylists). Senza busy, come
    // all'apertura: il pannello dell'account mostra il controllo in corso.
    async function reloadGoogleAccount() {
        if (!stateRef.current?.google?.connected) return
        setPrefsSyncing(true)
        try {
            const synced = await SyncSettings()
            absorbSynced(synced.state)
            const resp = await GooglePlaylists()
            absorbSynced(resp.state)
            if (!resp.ok) notify(false, resp.message ?? '')
        } catch (err: any) {
            notify(false, 'Errore: ' + (err?.message ?? String(err)))
        } finally {
            setPrefsSyncing(false)
        }
    }

    // Cambio di scheda: il contenuto riparte dall'alto.
    function selectSettingsTab(tab: SettingsTab) {
        setSettingsTab(tab)
        scrollRef.current?.scrollTo({ top: 0 })
    }

    // leaveSettings esce dalle Impostazioni dopo aver salvato le modifiche in
    // attesa, e riallinea le bozze a ciò che il core ha salvato (senza le righe
    // lasciate vuote).
    function leaveSettings() {
        flushSave().then(() => {
            setShowSettings(false)
            if (stateRef.current) syncDrafts(stateRef.current)
        })
    }

    // "Ripristina predefiniti" (dopo conferma): copia i predefiniti in regole e
    // playlist correnti e riallinea le bozze. Prima completa un eventuale
    // salvataggio in attesa, che altrimenti arriverebbe dopo e lo annullerebbe.
    function confirmResetConfig() {
        setConfirmReset(false)
        guard(async () => {
            await flushSave()
            const resp = await ResetConfig()
            absorb(resp, true)
            setResults(null)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Pulisce solo le attività stampate: non ripristina né salva altro stato (folder/regole/anteprima).
    async function clearLogs() {
        try {
            const resp = await ClearLogs()
            setState((prev) => (prev ? ({ ...prev, logs: resp.state.logs } as core.StateResponse) : prev))
        } catch {
            /* niente da fare: la pulizia log non deve disturbare lo stato */
        }
    }

    // "Salva predefiniti" (dopo conferma): rende regole e playlist correnti il
    // nuovo predefinito. Non tocca i valori correnti né le bozze: aggiorna solo
    // il registro.
    function confirmMakeDefault() {
        setConfirmDefault(false)
        guard(async () => {
            await flushSave()
            const cfg = draftRef.current
            if (!cfg) return
            const resp = await SetAsDefault(cfg, playlistDraftRef.current)
            setState((prev) => (prev ? ({ ...prev, logs: resp.state.logs } as core.StateResponse) : resp.state))
            notify(resp.ok, resp.message ?? '')
        })
    }

    const folder = state?.folder ?? ''
    folderRef.current = folder
    // simpleMode: modalità semplificata salvata, che decide la schermata
    // principale (solo scelta della playlist, "Scarica e converti", niente
    // anteprima).
    const simpleMode = !!state?.config?.simpleMode
    const files = state?.files ?? []
    const logs = state?.logs ?? []
    // Righe del registro arrivate dopo l'ultima apertura di Attività (le più
    // recenti sono in cima): se la riga vista non c'è più sono tutte nuove.
    const seenAt = logs.findIndex((l) => logKey(l) === logsSeen)
    const unseenLogs = seenAt === -1 ? logs : logs.slice(0, seenAt)
    const logsUnseen = unseenLogs.length === 0 ? 'none' : unseenLogs.some((l) => l.kind === 'error') ? 'error' : 'info'
    // Con il pannello aperto ogni riga che arriva è già vista.
    const topLogKey = logKey(logs[0])
    useEffect(() => {
        if (activityOpen) setLogsSeen(topLogKey)
    }, [activityOpen, topLogKey])
    // Il registro c'è solo nella schermata principale normale: uscendone si chiude.
    useEffect(() => {
        if (showSettings || simpleMode) setActivityOpen(false)
    }, [showSettings, simpleMode])
    const playlists = state?.playlists ?? []
    // Cosa scarica il tasto di download: il link, se inserito, altrimenti la
    // playlist selezionata (che intanto si disattiva).
    const link = downloadLink.trim()
    const canDownloadSource = link !== '' || selectedPlaylist !== ''
    const downloadWhat = link ? 'del link' : 'della playlist'
    // Account Google: googleAvailable = accesso possibile in questa build (su
    // desktop servono le credenziali OAuth). Con l'account collegato e il link
    // di un video, il campo del link offre l'aggiunta a una playlist.
    const googleAvailable = !!state?.google?.available
    const googleConnected = googleAvailable && !!state?.google?.connected
    const onAddLink = googleConnected && isVideoLink(link) ? () => openGooglePicker('add') : undefined
    // ID delle playlist già nella bozza: nel popup di importazione non si
    // possono riaggiungere.
    const savedPlaylistIds = new Set(playlistDraft.map((p) => playlistIdOf(p.url)).filter((id) => id !== ''))
    // Scelta della playlist da scaricare: prima quelle dell'account Google,
    // poi quelle salvate, senza le nascoste.
    const accountPlaylists = googleConnected ? (state?.google?.playlists ?? []) : []
    const playlistPrefs = state?.playlistPrefs ?? {}
    const playlistOptions = playlistChoices(accountPlaylists, playlists, playlistPrefs)
    const noPlaylists = playlistOptions.length === 0
    // Contatori nell'header: dopo un'elaborazione la lista `files` \u00e8 vuota
    // (i file sono stati rinominati/spostati), quindi mostreremmo "0 file".
    // Quando ci sono `results` calcoliamo i contatori da quelli, cos\u00ec l'utente
    // vede il riepilogo di ci\u00f2 che \u00e8 appena stato fatto.
    const showingResults = results !== null
    const fileCount = showingResults ? results!.length : files.length
    const mp3Count = showingResults
        ? results!.filter((r) => r.tagged).length
        : files.filter((f) => f.mp3).length
    const toRenameCount = showingResults
        ? results!.filter((r) => !r.skipped && !r.failed && r.oldName !== r.newName).length
        : files.filter((f) => f.preview !== f.name).length
    const failedCount = showingResults ? results!.filter((r) => r.failed).length : 0
    const destReady = destSameAsSource || destFolder !== ''
    // Cartelle impostate ma non trovate sul disco: si avverte e non si
    // scansiona/scarica/converte (lo rifiuta comunque anche il core). Su Android
    // senza accesso ai file sembrerebbero tutte mancanti: lì parla già il banner.
    const folderMissing = storageGranted && !!state?.folderMissing
    const destMissing = storageGranted && !destSameAsSource && !!state?.destinationMissing
    const foldersOk = folder !== '' && !folderMissing && !destMissing
    // Motivo per cui scansione e download non sono disponibili ('' se lo sono).
    const foldersHint = !folder
        ? 'Seleziona prima una cartella di partenza'
        : folderMissing
          ? 'Cartella di partenza non trovata: sceglila di nuovo nelle Impostazioni'
          : destMissing
            ? 'Cartella di destinazione non trovata: sceglila di nuovo nelle Impostazioni'
            : ''

    // Pull to refresh (Android): nella schermata principale (non semplificata,
    // senza anteprima) è una nuova scansione, come "Avvia nuova scansione";
    // nelle Impostazioni rilegge i dati dell'account Google, e senza account
    // collegato il gesto non fa nulla.
    const ptrRef = useRef<HTMLDivElement>(null)
    const pullEnabled =
        !busy && (showSettings ? googleConnected : !simpleMode && foldersOk && storageGranted)
    usePullToRefresh(scrollRef, ptrRef, !!state, pullEnabled, () => (showSettings ? reloadGoogleAccount() : refresh()))

    const canProcess = !busy && foldersOk && files.length > 0 && destReady
    // "Cancella tag" agisce in posto sugli MP3 scansionati: serve almeno un MP3.
    const canClearTags = !busy && files.some((f) => f.mp3)
    // Anteprima filtrata: se il toggle è attivo, mostra solo i file che
    // subiranno UNA QUALSIASI modifica — nel nome oppure nei tag ID3
    // (titolo/artista) — non solo quelli da rinominare. Resta comunque solo una
    // vista: l'elaborazione tratta sempre tutti i file.
    // I brani spuntati da rivedere contano come "da modificare": il filtro non
    // li nasconde, altrimenti resterebbero selezionati senza essere visibili.
    const previewFiles = showOnlyChanged
        ? files.filter((f) => fileWillChange(f) || reviewPaths.has(f.path))
        : files

    // A ogni nuova anteprima (scansione, aggiornamento automatico, conversione)
    // la selezione tiene solo i file ancora presenti.
    useEffect(() => {
        setReviewPaths((prev) => {
            if (prev.size === 0) return prev
            const present = new Set(files.map((f) => f.path))
            const next = new Set([...prev].filter((p) => present.has(p)))
            return next.size === prev.size ? prev : next
        })
    }, [files])

    function toggleReview(path: string, checked: boolean) {
        setReviewPaths((prev) => {
            const next = new Set(prev)
            if (checked) next.add(path)
            else next.delete(path)
            return next
        })
    }

    // Casella nell'intestazione: seleziona/deseleziona tutti i file visibili
    // (con "Solo da modificare" solo quelli mostrati).
    const allVisibleSelected = previewFiles.length > 0 && previewFiles.every((f) => reviewPaths.has(f.path))
    const someVisibleSelected = previewFiles.some((f) => reviewPaths.has(f.path))
    function toggleReviewAll(checked: boolean) {
        setReviewPaths((prev) => {
            const next = new Set(prev)
            for (const f of previewFiles) {
                if (checked) next.add(f.path)
                else next.delete(f.path)
            }
            return next
        })
    }
    // Etichetta della barra di avanzamento: in modalità semplificata download e
    // conversione si susseguono nella stessa operazione, quindi diciamo quale
    // delle due fasi è in corso.
    const progressLabel = progress
        ? (simpleMode ? (progress.phase === 'convert' ? 'Conversione · ' : 'Download · ') : '') +
          `${progress.done} / ${progress.total} completati`
        : ''

    // Attiva/disattiva "Elimina originali" con conferma esplicita quando si passa
    // da OFF a ON (è un'azione distruttiva). Spegnerlo non richiede conferma.
    function toggleDeleteOriginals(next: boolean) {
        if (next && !deleteOriginals) {
            setConfirmDeleteOriginals(true)
            return
        }
        applyOptions(destSameAsSource, destFolder, next)
    }

    function confirmEnableDelete() {
        setConfirmDeleteOriginals(false)
        applyOptions(destSameAsSource, destFolder, true)
    }

    function updateDraftList(
        key:
            | 'supportedExtensions'
            | 'occurrenciesToRemove'
            | 'occurrenciesToReplaceWithFt'
            | 'artistExceptions',
        values: string[],
    ) {
        if (!draft) return
        setDraft({ ...draft, [key]: values } as rules.Config)
    }

    function updateFtAlias(value: string) {
        if (!draft) return
        setDraft({ ...draft, ftAlias: value } as rules.Config)
    }

    // Le sostituzioni sono mostrate divise per ambito (REPLACEMENT_SCOPES) ma
    // restano un unico elenco: la pipeline le applica in ordine, quindi ogni
    // riga resta al suo posto (index è quello nell'elenco completo) e le nuove
    // vanno in fondo.
    function updateReplacement(index: number, field: 'from' | 'to', value: string) {
        if (!draft) return
        const replacements = (draft.replacements ?? []).map((r, i) => (i === index ? { ...r, [field]: value } : r))
        setDraft({ ...draft, replacements } as rules.Config)
    }

    function addReplacement(scope: string) {
        if (!draft) return
        const row = (scope ? { from: '', to: '', scope } : { from: '', to: '' }) as rules.Replacement
        setDraft({ ...draft, replacements: [...(draft.replacements ?? []), row] } as rules.Config)
    }

    function removeReplacement(index: number) {
        if (!draft) return
        const replacements = (draft.replacements ?? []).filter((_, i) => i !== index)
        setDraft({ ...draft, replacements } as rules.Config)
    }

    // Playlist YouTube (Impostazioni): stessa logica di editing delle
    // sostituzioni Da→A, ma su un elenco a parte (playlistDraft) salvato con
    // SetPlaylists, non con SetConfig.
    function updatePlaylistDraft(index: number, field: 'name' | 'url', value: string) {
        setPlaylistDraft((prev) => prev.map((p, i) => (i === index ? { ...p, [field]: value } : p)))
    }

    function addPlaylistDraft() {
        setPlaylistDraft((prev) => [...prev, { name: '', url: '' }])
    }

    function removePlaylistDraft(index: number) {
        setPlaylistDraft((prev) => prev.filter((_, i) => i !== index))
    }

    // Collega l'account Google: su desktop l'accesso si completa nel browser
    // (il pannello mostra "Annulla", che ferma l'attesa del core), su Android
    // nella schermata di Google Play Services.
    function googleSignIn() {
        setGoogleSigningIn(true)
        setCancellable(true)
        guard(async () => {
            const resp = await GoogleSignIn()
            absorb(resp)
            notify(resp.ok, resp.message ?? '')
        }).finally(() => {
            setGoogleSigningIn(false)
            setCancellable(false)
        })
    }

    function googleSignOut() {
        setConfirmGoogleSignOut(false)
        guard(async () => {
            const resp = await GoogleSignOut()
            absorb(resp)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Apre il popup delle playlist dell'account e ne carica l'elenco (il
    // caricamento si vede nel popup, senza busy). Se l'accesso non è più valido
    // il core scollega l'account e il popup si chiude con l'errore.
    async function openGooglePicker(mode: 'import' | 'add') {
        const req = (pickerReqRef.current += 1)
        setGooglePicker(mode)
        setGoogleLists(null)
        setGoogleImport(new Set())
        try {
            const resp = await GooglePlaylists()
            if (req !== pickerReqRef.current) return
            if (!resp.ok) {
                setGooglePicker(null)
                setState((prev) => (prev ? ({ ...prev, google: resp.state.google, logs: resp.state.logs } as core.StateResponse) : prev))
                notify(false, resp.message ?? '')
                return
            }
            setGoogleLists(resp.googlePlaylists ?? [])
        } catch (err: any) {
            if (req !== pickerReqRef.current) return
            setGooglePicker(null)
            notify(false, 'Errore: ' + (err?.message ?? String(err)))
        }
    }

    function closeGooglePicker() {
        pickerReqRef.current += 1
        setGooglePicker(null)
    }

    // Aggiunge alla bozza le playlist spuntate (le righe vuote si tolgono). Il
    // nome è il titolo su YouTube, reso unico: il download le cerca per nome.
    function importGooglePlaylists() {
        const chosen = (googleLists ?? []).filter((p) => googleImport.has(p.id))
        setPlaylistDraft((prev) => {
            const rows = prev.filter((p) => p.name.trim() !== '' || p.url.trim() !== '')
            const names = new Set(rows.map((p) => p.name.trim()))
            for (const p of chosen) {
                let name = p.title.trim() || p.id
                for (let n = 2; names.has(name); n++) name = `${p.title.trim() || p.id} (${n})`
                names.add(name)
                rows.push({ name, url: p.url })
            }
            return rows
        })
        closeGooglePicker()
        notify(true, chosen.length === 1 ? '1 playlist importata.' : `${chosen.length} playlist importate.`)
    }

    // Apre le impostazioni della playlist con chiave key.
    function openPlaylistPrefs(key: string, title: string) {
        const id = key.startsWith('yt:') ? key.slice(3) : ''
        const ownId = id && accountPlaylists.some((p) => p.id === id) ? id : ''
        setPrefsDraft({ ...(playlistPrefs[key] ?? {}) })
        setPrefsTarget({ key, title, ownId })
    }

    // Le impostazioni delle playlist cambiano anche la scelta del download
    // (le nascoste spariscono): si riallinea la playlist selezionata.
    function absorbPrefs(next: core.StateResponse) {
        setState((prev) =>
            prev ? ({ ...prev, playlistPrefs: next.playlistPrefs, logs: next.logs } as core.StateResponse) : prev,
        )
        if (stateRef.current) {
            syncSelectedPlaylist({ ...stateRef.current, playlistPrefs: next.playlistPrefs } as core.StateResponse)
        }
    }

    function savePlaylistPrefs() {
        const target = prefsTarget
        if (!target) return
        const draft = { ...prefsDraft }
        if (draft.afterDownload !== 'copy') {
            delete draft.copyTo
            delete draft.copyToTitle
            delete draft.moveOnCopy
        }
        if (!target.ownId) delete draft.moveOnCopy
        guard(async () => {
            const resp = await SetPlaylistPrefs(target.key, draft as playlist.Prefs)
            absorbPrefs(resp.state)
            if (resp.ok) setPrefsTarget(null)
            notify(resp.ok, resp.ok ? 'Impostazioni della playlist salvate.' : (resp.message ?? ''))
        })
    }

    // «Svuota ora» (dopo conferma): toglie da YouTube tutti i brani della
    // playlist, con avanzamento e Annulla nel riquadro delle playlist.
    function emptyPlaylist() {
        const target = confirmEmpty
        if (!target) return
        setConfirmEmpty(null)
        setPrefsTarget(null)
        setEmptying(target.id)
        setProgress(null)
        setCancellable(true)
        guard(async () => {
            const resp = await EmptyGooglePlaylist(target.id)
            absorbSynced(resp.state)
            notify(resp.ok, resp.message ?? '')
        }).finally(() => {
            setEmptying('')
            setCancellable(false)
            setProgress(null)
        })
    }

    function addLinkToPlaylist(p: core.GooglePlaylistView) {
        closeGooglePicker()
        guard(async () => {
            const resp = await AddLinkToPlaylist(link, p.id, p.title)
            setState((prev) => (prev ? ({ ...prev, google: resp.state.google, logs: resp.state.logs } as core.StateResponse) : prev))
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Avvia il download del link inserito o, se il campo è vuoto, della
    // playlist selezionata. Se yt-dlp non è presente
    // chiede prima conferma con un popup: se la gestione automatica è attiva
    // propone solo di scaricarlo, altrimenti propone di attivarla e procedere
    // (in entrambi i casi lo scarica, insieme a ffmpeg se manca, e poi prosegue).
    // Se c'è yt-dlp ma manca ffmpeg, propone di scaricare solo ffmpeg. Se ci
    // sono entrambi, scarica direttamente nella cartella di partenza (la
    // scansione riparte automaticamente lato backend).
    function downloadPlaylist() {
        if (!canDownloadSource) return
        // In modalità semplificata la conversione segue il download: la
        // destinazione deve esserci già (il core la ricontrolla comunque).
        if (simpleMode && !destReady) {
            notify(false, 'Scegli una cartella di destinazione nelle Impostazioni o riattiva "uguale alla partenza".')
            return
        }
        if (!state?.ytDlpAvailable) {
            setConfirmInstallYtDlp(true)
            return
        }
        if (!state?.ffmpegAvailable) {
            setConfirmFFmpeg('playlist')
            return
        }
        runPlaylistOp(playlistStep)
    }

    // runPlaylistOp esegue `fn` (il download della playlist, eventualmente
    // preceduto dall'installazione di yt-dlp/ffmpeg) in un unico "busy". Il
    // download è annullabile (Cancel) e riporta l'avanzamento (canzoni scaricate
    // / totale) via gli eventi process:progress: azzeriamo il contatore e
    // mostriamo il tasto Annulla + la barra di avanzamento.
    function runPlaylistOp(fn: () => Promise<void>) {
        setProgress(null)
        setDownloadErrors([])
        if (simpleMode) {
            setResults(null)
            setTagPrompts([])
        }
        setCancellable(true)
        guard(fn).finally(() => {
            setProgress(null)
            setCancellable(false)
        })
    }

    // playlistStep scarica il link inserito (o la playlist selezionata) e ne
    // assorbe l'esito. In modalità semplificata (DownloadAndProcess) il core
    // converte anche subito i brani: mostriamo i risultati e le tracce da
    // confermare come dopo "Converti nomi e scrivi tag". Un link scaricato senza
    // errori si toglie dal campo, a meno che nel frattempo non sia cambiato.
    async function playlistStep() {
        const [kind, key] = [selectedPlaylist.slice(0, 3), selectedPlaylist.slice(3)]
        const resp = link
            ? await (simpleMode ? DownloadLinkAndProcess(link) : DownloadLink(link))
            : kind === 'yt:'
              ? await (simpleMode ? DownloadGooglePlaylistAndProcess(key) : DownloadGooglePlaylist(key))
              : await (simpleMode ? DownloadAndProcess(key) : DownloadPlaylist(key))
        setCancellable(false)
        absorb(resp)
        if (link && resp.ok) setDownloadLink((prev) => (prev.trim() === link ? '' : prev))
        setDownloadErrors(resp.downloadErrors ?? [])
        notify(resp.ok, resp.message ?? '')
        if (simpleMode) {
            setResults(resp.results?.length ? resp.results : null)
            setTagPrompts(promptsOf(resp))
        } else {
            setResults(null)
        }
    }

    // Conferma dal popup: se la gestione automatica non è attiva la attiva prima
    // (solo così l'app può scaricare la propria copia in %AppData%), poi scarica
    // yt-dlp e, se va a buon fine, procede col download della playlist: tutto in
    // un unico "busy".
    function confirmInstallThenDownload() {
        setConfirmInstallYtDlp(false)
        runPlaylistOp(async () => {
            if (!ytDlpManaged) {
                const cfg = await SetYtDlpConfig(true, ytDlpPathDraft)
                absorb(cfg)
                syncOptions(cfg.state)
                if (!cfg.ok) {
                    notify(false, cfg.message ?? '')
                    return
                }
            }
            const inst = await InstallYtDlp()
            setInstallProgress(null)
            absorb(inst)
            syncOptions(inst.state)
            notify(inst.ok, inst.message ?? '')
            if (!inst.ok) return
            // InstallYtDlp scarica solo yt-dlp: se manca anche ffmpeg (serve per
            // gli mp3) lo scarichiamo qui, come annunciato dal popup.
            if (!isAndroid && !inst.state.ffmpegAvailable) {
                const ff = await InstallFFmpeg()
                setInstallProgress(null)
                absorb(ff)
                syncOptions(ff.state)
                notify(ff.ok, ff.message ?? '')
                if (!ff.ok) return
            }
            await playlistStep()
        })
    }

    // Attiva/disattiva la gestione automatica di yt-dlp. In gestione automatica
    // l'app usa/aggiorna la propria copia in %AppData%; altrimenti si usa il
    // percorso personalizzato correntemente nel campo. È un semplice cambio di
    // impostazione: niente busy a tutta UI (che farebbe sembrare la checkbox
    // lenta): flippiamo subito in modo ottimistico e persistiamo in background,
    // assorbendo lo stato reale al ritorno. Riattivandola, finché il backend
    // cerca la copia locale di yt-dlp, al posto del badge compare un loader
    // (per almeno MIN_CHECK_MS, così non lampeggia se la ricerca è istantanea).
    function toggleYtDlpManaged(next: boolean) {
        const MIN_CHECK_MS = 700
        const started = Date.now()
        setYtDlpManaged(next)
        if (next) setYtDlpChecking(true)
        SetYtDlpConfig(next, ytDlpPathDraft)
            .then((resp) => {
                absorb(resp)
                syncOptions(resp.state)
                notify(resp.ok, resp.message ?? '')
            })
            .catch((e) => notify(false, String(e)))
            .finally(async () => {
                if (!next) return
                const elapsed = Date.now() - started
                if (elapsed < MIN_CHECK_MS) {
                    await new Promise((r) => window.setTimeout(r, MIN_CHECK_MS - elapsed))
                }
                setYtDlpChecking(false)
            })
    }

    // Persiste il percorso personalizzato (all'uscita dal campo): disattiva la
    // gestione automatica, dato che si sta puntando a un eseguibile scelto a mano.
    function applyYtDlpPath() {
        guard(async () => {
            const resp = await SetYtDlpConfig(false, ytDlpPathDraft)
            absorb(resp)
            syncOptions(resp.state)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Selettore file per scegliere l'eseguibile yt-dlp personalizzato; alla
    // conferma imposta il percorso e disattiva la gestione automatica.
    function browseYtDlp() {
        guard(async () => {
            const path = await ChooseYtDlpFile()
            if (!path) return
            setYtDlpPathDraft(path)
            const resp = await SetYtDlpConfig(false, path)
            absorb(resp)
            syncOptions(resp.state)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Scarica/installa yt-dlp nel percorso effettivo in uso (copia gestita in
    // %AppData% se la gestione automatica è attiva, altrimenti il percorso
    // personalizzato). Usato dal tasto di download mostrato quando yt-dlp non è
    // presente. Il backend risponde con un errore chiaro se manca un percorso.
    function installYtDlp() {
        setConfirmDownloadYtDlp(false)
        guard(async () => {
            const resp = await InstallYtDlp()
            setInstallProgress(null)
            absorb(resp)
            syncOptions(resp.state)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Scarica ffmpeg nella copia gestita dall'app (%AppData%\RenameMusic\ffmpeg),
    // dopo conferma. Se la richiesta nasce dal "Scarica" di una playlist, a
    // installazione riuscita prosegue col download della playlist (un solo busy).
    function installFFmpeg() {
        const thenDownload = confirmFFmpeg === 'playlist'
        setConfirmFFmpeg(null)
        if (!thenDownload) {
            guard(async () => {
                const resp = await InstallFFmpeg()
                setInstallProgress(null)
                absorb(resp)
                syncOptions(resp.state)
                notify(resp.ok, resp.message ?? '')
            })
            return
        }
        runPlaylistOp(async () => {
            const inst = await InstallFFmpeg()
            setInstallProgress(null)
            absorb(inst)
            syncOptions(inst.state)
            notify(inst.ok, inst.message ?? '')
            if (!inst.ok) return
            await playlistStep()
        })
    }

    // Rimuove la copia di yt-dlp gestita dall'app (%AppData%\RenameMusic), dopo
    // conferma. Ha senso solo in gestione automatica: in modalità manuale il file
    // è dell'utente e il backend rifiuta la rimozione.
    function uninstallYtDlp() {
        setConfirmUninstallYtDlp(false)
        guard(async () => {
            const resp = await UninstallYtDlp()
            absorb(resp)
            syncOptions(resp.state)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Rimuove la copia di ffmpeg gestita dall'app, dopo conferma. Un ffmpeg di
    // sistema non viene toccato (e resta in uso, se c'è).
    function uninstallFFmpeg() {
        setConfirmUninstallFFmpeg(false)
        guard(async () => {
            const resp = await UninstallFFmpeg()
            absorb(resp)
            syncOptions(resp.state)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Controllo manuale degli aggiornamenti (Impostazioni): l'esito va sempre
    // in un toast, anche quando l'app è già aggiornata.
    function checkUpdate() {
        guard(async () => {
            const resp = await CheckUpdate()
            absorbUpdate(resp.state)
            notify(resp.ok, resp.message ?? '')
        })
    }

    // Scarica e installa la nuova versione: su desktop l'app si riavvia da sola,
    // su Android si apre l'installer di sistema. In caso di errore il popup
    // resta aperto per poter riprovare.
    function installUpdate() {
        guard(async () => {
            const resp = await InstallUpdate()
            setInstallProgress(null)
            absorbUpdate(resp.state)
            notify(resp.ok, resp.message ?? '')
            if (resp.ok) setUpdatePopup(null)
        })
    }

    // Popup del crash: "Condividi errore" apre il menu di condivisione di
    // Android (es. WhatsApp) con il file di testo; entrambe le scelte eliminano
    // il registro, così il popup non ricompare.
    function shareCrash() {
        setCrashReport(false)
        shareCrashReport().catch((err: any) => notify(false, 'Condivisione non riuscita: ' + (err?.message ?? String(err))))
    }

    function discardCrash() {
        setCrashReport(false)
        discardCrashReport().catch(() => {})
    }

    // closeTopmost chiude, in ordine, la modale aperta o il pannello
    // impostazioni (Esc su desktop, tasto Indietro su Android). Restituisce
    // false se non c'era nulla da chiudere.
    function closeTopmost(): boolean {
        if (crashReport) {
            // Indietro lo chiude senza eliminare: ricompare al prossimo avvio.
            setCrashReport(false)
        } else if (updatePopup) {
            if (!busy) setUpdatePopup(null)
        } else if (showDownloadErrors) setShowDownloadErrors(false)
        else if (confirmEmpty) setConfirmEmpty(null)
        else if (prefsTarget) setPrefsTarget(null)
        else if (googlePicker) setGooglePicker(null)
        else if (confirmGoogleSignOut) setConfirmGoogleSignOut(false)
        else if (confirmDeleteOriginals) setConfirmDeleteOriginals(false)
        else if (confirmClearTags) setConfirmClearTags(false)
        else if (confirmInstallYtDlp) setConfirmInstallYtDlp(false)
        else if (confirmUninstallYtDlp) setConfirmUninstallYtDlp(false)
        else if (confirmUninstallFFmpeg) setConfirmUninstallFFmpeg(false)
        else if (confirmDownloadYtDlp) setConfirmDownloadYtDlp(false)
        else if (confirmFFmpeg) setConfirmFFmpeg(null)
        else if (confirmReset) setConfirmReset(false)
        else if (confirmDefault) setConfirmDefault(false)
        else if (defaultsMenu) setDefaultsMenu(false)
        else if (activityOpen) setActivityOpen(false)
        else if (showSettings) leaveSettings()
        else return false
        return true
    }

    // Tasto Indietro di Android: chiude modali/impostazioni come Esc; se non
    // c'è nulla da chiudere manda l'app in background (non la chiude, così
    // un'operazione in corso prosegue). Con il selettore cartelle aperto è lui a
    // gestire Indietro (risale di cartella), e il popup delle tracce da
    // correggere richiede una scelta esplicita. Il gestore vive in un ref, come
    // le scorciatoie, per vedere sempre lo stato corrente.
    const backRef = useRef<() => void>(() => {})
    backRef.current = () => {
        if (folderPicker || tagPrompts.length > 0) return
        if (!closeTopmost()) CapApp.minimizeApp().catch(() => {})
    }
    useEffect(() => {
        if (!isAndroid) return
        const handle = CapApp.addListener('backButton', () => backRef.current())
        return () => {
            handle.then((h) => h.remove())
        }
    }, [])

    // Scorciatoie da tastiera. Il gestore è tenuto in un ref aggiornato ad ogni
    // render, così il listener (registrato una sola volta) vede sempre lo stato
    // corrente senza doversi ri-registrare ad ogni cambiamento.
    const shortcutRef = useRef<(e: KeyboardEvent) => void>(() => {})
    shortcutRef.current = (e: KeyboardEvent) => {
        // Non intercettare mentre si scrive in un campo di testo: lì Esc fa solo
        // uscire dal campo (un secondo Esc chiude, come sempre). Le checkbox non
        // contano: dopo averne spuntata una il primo Esc deve già chiudere.
        const target = e.target as HTMLElement | null
        const isTextField =
            target instanceof HTMLInputElement
                ? target.type !== 'checkbox' && target.type !== 'radio'
                : !!target && (target.tagName === 'TEXTAREA' || target.isContentEditable)
        if (target && isTextField) {
            if (e.key === 'Escape') target.blur()
            return
        }

        // Esc chiude, in ordine: modali aperte, poi il pannello impostazioni.
        if (e.key === 'Escape') {
            closeTopmost()
            return
        }

        if (!e.ctrlKey) return
        switch (e.key.toLowerCase()) {
            // Ctrl+I: cartella di partenza; Ctrl+O: cartella di destinazione, solo se
            // distinta dalla partenza. Nella schermata normale e nelle Impostazioni;
            // in modalità semplificata solo nelle Impostazioni.
            case 'i':
                if (busy || (simpleMode && !showSettings)) return
                e.preventDefault()
                chooseFolder()
                break
            case 'o':
                if (busy || (simpleMode && !showSettings) || destSameAsSource) return
                e.preventDefault()
                chooseDestination()
                break
            case 'r': // Aggiorna scansione (in modalità semplificata non c'è anteprima)
                if (busy || !foldersOk || simpleMode) return
                e.preventDefault()
                refresh()
                break
            case 'enter': // Converti (o, nella vista risultati, nuova scansione); in modalità semplificata "Scarica e converti"
                e.preventDefault()
                if (simpleMode) {
                    if (!busy && !showSettings && canDownloadSource && foldersOk) downloadPlaylist()
                } else if (results) {
                    if (!busy && foldersOk) refresh()
                } else if (canProcess) {
                    process()
                }
                break
            case ',': // Mostra/nascondi impostazioni
                if (busy) return
                e.preventDefault()
                if (showSettings) leaveSettings()
                else openSettings()
                break
            case '1': // Ctrl+1…4: scheda delle Impostazioni
            case '2':
            case '3':
            case '4':
                if (!showSettings) return
                e.preventDefault()
                selectSettingsTab(SETTINGS_TABS[Number(e.key) - 1].id)
                break
            case 'arrowleft': // Ctrl+←/→: scheda precedente/successiva (in giro)
            case 'arrowright': {
                if (!showSettings) return
                e.preventDefault()
                const step = e.key === 'ArrowRight' ? 1 : -1
                const current = SETTINGS_TABS.findIndex((t) => t.id === settingsTab)
                selectSettingsTab(SETTINGS_TABS[(current + step + SETTINGS_TABS.length) % SETTINGS_TABS.length].id)
                break
            }
        }
    }
    useEffect(() => {
        const handler = (e: KeyboardEvent) => shortcutRef.current(e)
        window.addEventListener('keydown', handler)
        return () => window.removeEventListener('keydown', handler)
    }, [])

    // Barra di avanzamento dell'operazione in corso, sotto i comandi (nella
    // testata o nella card della modalità semplificata). Durante "Scarica" di una
    // playlist può mostrare prima il download di yt-dlp/ffmpeg, se mancavano.
    const opProgress =
        busy && progress && progress.total > 0 ? (
            <OpProgress percent={Math.round((progress.done / progress.total) * 100)} label={progressLabel} />
        ) : busy && installProgress ? (
            <OpProgress percent={installPercent(installProgress)} label={installLabel(installProgress)} />
        ) : null

    // Cartelle di partenza/destinazione e opzioni di conversione: stanno sempre
    // nelle Impostazioni (scheda Generale), in qualunque modalità. Si applicano
    // subito e non fanno parte dei predefiniti.
    const folderSettings = (
        <>
            <div className="field-group">
                <span className="field-label">Cartella di partenza</span>
                <div className="toolbar">
                    <div className="folder-path">
                        {folder || 'Nessuna cartella selezionata'}
                    </div>
                    {folderMissing && <MissingFolderIcon label="Cartella di partenza" />}
                    {!isAndroid && (
                        <Tooltip label="Apri la cartella in Esplora risorse">
                            <button
                                className="ghost with-icon"
                                onClick={() => openFolder(folder)}
                                disabled={busy || !folder || folderMissing}
                            >
                                <span className="btn-icon"><FolderOpenIcon /></span>
                                Apri
                            </button>
                        </Tooltip>
                    )}
                    <button className="primary" onClick={chooseFolder} disabled={busy || !storageGranted}>
                        Scegli cartella
                    </button>
                </div>
            </div>

            {!destSameAsSource && (
                <div className="field-group">
                    <span className="field-label">Cartella di destinazione</span>
                    <div className="toolbar">
                        <div className="folder-path">
                            {destFolder || 'Nessuna destinazione selezionata'}
                        </div>
                        {destMissing && <MissingFolderIcon label="Cartella di destinazione" />}
                        {!isAndroid && (
                            <Tooltip label="Apri la cartella in Esplora risorse">
                                <button
                                    className="ghost with-icon"
                                    onClick={() => openFolder(destFolder)}
                                    disabled={busy || !destFolder || destMissing}
                                >
                                    <span className="btn-icon"><FolderOpenIcon /></span>
                                    Apri
                                </button>
                            </Tooltip>
                        )}
                        <button className="primary" onClick={chooseDestination} disabled={busy || !storageGranted}>
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
                    onChange={(checked) => applyOptions(checked, destFolder, deleteOriginals)}
                    disabled={busy}
                />

                <CheckOption
                    label="Eliminazione file originali"
                    info="Quando attiva, dopo la conversione i file di partenza vengono eliminati definitivamente dal disco. Quando disattivata, i nuovi file convertiti vengono scritti senza toccare gli originali."
                    checked={deleteOriginals}
                    onChange={toggleDeleteOriginals}
                    disabled={busy}
                />
            </div>
        </>
    )

    // Riepilogo delle cartelle nella schermata principale: dove si
    // leggono/finiscono i brani (si scelgono solo nelle Impostazioni).
    const folderSummary = (
        <>
            <FolderLines
                folder={folder}
                destSameAsSource={destSameAsSource}
                destFolder={destFolder}
                folderMissing={folderMissing}
                destMissing={destMissing}
                onMissingClick={() => openSettings('general')}
                disabled={busy}
            />
        </>
    )

    // Finché non arriva il primo stato (GetConfig, pochi ms) non sappiamo quale
    // schermata disegnare (normale o semplificata): meglio un istante vuoto che
    // la schermata sbagliata che poi cambia.
    if (!state) return <div className={'app' + (isAndroid ? ' is-android' : '')} />

    return (
        <div className={'app' + (isAndroid ? ' is-android' : '')}>
            {/* Barra delle Impostazioni: al posto dell'header, fissa in cima. Le
                modifiche si salvano da sole: qui restano solo l'uscita e i
                predefiniti (regole, playlist e modalità semplificata). */}
            {showSettings && (
            <header className="settings-header">
                <div className="header-inner">
                    <div className="settings-header-title">
                        <button
                            type="button"
                            className="header-btn settings-back"
                            onClick={leaveSettings}
                            disabled={busy}
                            aria-label="Indietro"
                        >
                            <BackIcon />
                            <span className="btn-label">Indietro</span>
                        </button>
                        <h1>Impostazioni</h1>
                        {saveStatus && (
                            <span className="save-status" role="status">
                                {/* Su Android, per stare sulla riga con
                                    "Predefiniti", sul telefono resta solo
                                    l'icona (spinner o spunta, mobile.css). */}
                                {saveStatus === 'saving' ? (
                                    <>
                                        {isAndroid && <span className="spinner" aria-hidden="true" />}
                                        <span className="save-status-text">Salvataggio…</span>
                                    </>
                                ) : (
                                    <>
                                        <CheckIcon />
                                        <span className="save-status-text">Salvato</span>
                                    </>
                                )}
                            </span>
                        )}
                    </div>
                    <div className="settings-header-actions">
                        {isAndroid ? (
                            <DefaultsMenu
                                open={defaultsMenu}
                                onOpenChange={setDefaultsMenu}
                                onReset={() => setConfirmReset(true)}
                                onSave={() => setConfirmDefault(true)}
                                disabled={busy}
                            />
                        ) : (
                        <>
                        <ShortcutsLegend simple={simpleMode} separateDest={!destSameAsSource} settings />
                        <Tooltip label="Riporta regole, playlist e modalità semplificata ai predefiniti salvati.">
                            <button type="button" className="header-btn" onClick={() => setConfirmReset(true)} disabled={busy}>
                                Ripristina predefiniti
                            </button>
                        </Tooltip>
                        <Tooltip label="I predefiniti sono una configurazione di riserva, da recuperare con «Ripristina predefiniti». Salva come predefiniti regole, playlist e modalità semplificata attuali.">
                            <button
                                type="button"
                                className="header-btn warn-solid"
                                onClick={() => setConfirmDefault(true)}
                                disabled={busy}
                            >
                                Salva predefiniti
                            </button>
                        </Tooltip>
                        </>
                        )}
                    </div>
                </div>
            </header>
            )}

            {!showSettings && (
            <header>
                <div className="header-inner">
                <h1>RenameMusic</h1>
                <div className="header-right">
                    {!isAndroid && <ShortcutsLegend simple={simpleMode} separateDest={!destSameAsSource} />}
                    {!showSettings && (
                        <>
                            {/* Senza anteprima l'aggiornamento automatico non ha
                                nulla da aggiornare: in modalità semplificata il
                                toggle sparisce (e il core ne ignora gli eventi). */}
                            {!simpleMode && (
                            <Tooltip
                                label={
                                    !folder || folderMissing
                                        ? foldersHint
                                        : watchEnabled
                                          ? "Aggiornamento automatico attivo: clicca per disattivarlo. Le variazioni nella cartella aggiornano l'anteprima."
                                          : "Aggiornamento automatico disattivato: clicca per attivarlo e aggiornare l'anteprima automaticamente."
                                }
                            >
                                <button
                                    type="button"
                                    className={
                                        'watch-toggle' +
                                        (watchEnabled ? (folderMissing ? ' is-error' : ' is-on') : '')
                                    }
                                    onClick={() => toggleWatch(!watchEnabled)}
                                    disabled={busy || !folder || folderMissing}
                                    aria-pressed={watchEnabled}
                                >
                                    <span className="watch-dot" aria-hidden="true" />
                                    {watchEnabled ? 'Agg. automatico attivo' : 'Agg. automatico'}
                                </button>
                            </Tooltip>
                            )}
                            {/* In modalità semplificata i contatori riassumono solo
                                l'ultima conversione (non c'è un'anteprima da contare). */}
                            {(!simpleMode || showingResults) && (
                            <div className="counters">
                                <span>{fileCount} file{showingResults ? ' elaborati' : ''}</span>
                                <span className="dot">·</span>
                                <span>{mp3Count} MP3</span>
                                {!showingResults && reviewPaths.size > 0 && (
                                    <>
                                        <span className="dot">·</span>
                                        <span className="counter-review">{reviewPaths.size} da rivedere</span>
                                    </>
                                )}
                                {toRenameCount > 0 && (
                                    <>
                                        <span className="dot">·</span>
                                        <span className="counter-hi">
                                            {toRenameCount} {showingResults ? 'rinominati' : 'da rinominare'}
                                        </span>
                                    </>
                                )}
                                {failedCount > 0 && (
                                    <>
                                        <span className="dot">·</span>
                                        <span className="counter-err">{failedCount} errori</span>
                                    </>
                                )}
                            </div>
                            )}
                            {/* Registro Attività (non in modalità semplificata,
                                dove gli esiti arrivano solo dai toast). */}
                            {!simpleMode && (
                                <ActivityMenu
                                    open={activityOpen}
                                    onOpenChange={setActivityOpen}
                                    logs={logs}
                                    unseen={logsUnseen}
                                    onClear={clearLogs}
                                    clearDisabled={busy}
                                />
                            )}
                            <button
                                type="button"
                                className="header-btn with-icon"
                                onClick={() => openSettings(state?.update ? 'info' : undefined)}
                                disabled={busy}
                                aria-label={state?.update ? 'Impostazioni (aggiornamento disponibile)' : 'Impostazioni'}
                            >
                                <span className="btn-icon"><SettingsIcon /></span>
                                <span className="btn-label">Impostazioni</span>
                                {state?.update && <span className="update-dot" aria-hidden="true" />}
                            </button>
                        </>
                    )}
                </div>
                </div>
            </header>
            )}

            {/* Schede delle Impostazioni: tab orizzontali su desktop, pillole
                scorrevoli su Android (mobile.css). Stanno fuori dall'area che
                scorre, ferme sotto l'header: la scrollbar parte sotto di loro. */}
            {showSettings && draft && (
                <div className="settings-tabs-bar">
                    <nav className="settings-tabs" aria-label="Sezioni delle impostazioni">
                        {SETTINGS_TABS.map((t) => (
                            <button
                                key={t.id}
                                type="button"
                                className={'settings-tab' + (settingsTab === t.id ? ' is-active' : '')}
                                aria-current={settingsTab === t.id ? 'page' : undefined}
                                onClick={() => selectSettingsTab(t.id)}
                            >
                                {t.icon}
                                <span>{t.label}</span>
                                {t.id === 'info' && state?.update && <span className="update-dot" aria-hidden="true" />}
                            </button>
                        ))}
                    </nav>
                </div>
            )}

            {/* Scorre solo il contenuto sotto l'header (e sotto le schede delle
                Impostazioni): la scrollbar parte da qui e non dalla cima della
                finestra. */}
            {/* Indicatore del pull to refresh (Android): sta appena sotto
                l'header (e le schede) e scende con il dito (usePullToRefresh). */}
            {isAndroid && (
                <div className="ptr" aria-hidden="true">
                    <div className="ptr-indicator" ref={ptrRef}>
                        <RefreshIcon />
                    </div>
                </div>
            )}

            <div className="app-scroll" ref={scrollRef}>
            <main className={showSettings ? 'is-settings' : simpleMode ? 'is-simple' : ''}>
                <div
                    className={'busy-bar' + (busy ? ' is-active' : '')}
                    role="progressbar"
                    aria-hidden={!busy}
                    aria-label="Operazione in corso"
                />

                {!showSettings && !storageGranted && (
                    <div className="storage-banner" role="alert">
                        <div className="storage-banner-text">
                            <strong>Serve l'accesso ai file</strong>
                            <span>
                                Per leggere, rinominare e scaricare i brani l'app deve poter accedere
                                alle cartelle della memoria. Attiva "Consenti l'accesso per gestire
                                tutti i file" nella schermata che si apre, poi torna qui.
                            </span>
                        </div>
                        <button className="accent" onClick={askStorage}>
                            Concedi accesso
                        </button>
                    </div>
                )}

                {!showSettings && !simpleMode && (
                <div className="top-row">
                    <div className="top-left-head">
                        <div className="folder-summary">{folderSummary}</div>

                        <div className="actions">
                            {/* Link sopra la riga playlist + "Scarica", su una riga
                                propria e largo quanto lei. Il link, se inserito, ha
                                la precedenza: la playlist si chiude (animata) e
                                "Scarica" ne prende il posto. */}
                            <div
                                className="download-box"
                                style={downloadRowWidth ? ({ '--download-row-width': downloadRowWidth + 'px' } as CSSProperties) : undefined}
                            >
                            <LinkField
                                value={downloadLink}
                                onChange={setDownloadLink}
                                onSubmit={() => foldersOk && downloadPlaylist()}
                                onAddToPlaylist={onAddLink}
                                onPaste={isAndroid ? pasteLink : undefined}
                                disabled={busy}
                            />
                            <div className="download-controls" ref={downloadRowRef}>
                                <Collapse className="playlist-pick" collapsed={link !== ''} onSettled={measureDownloadRow}>
                                    <Tooltip label={noPlaylists ? 'Nessuna playlist: aggiungine una dalle Impostazioni o collega il tuo account Google' : 'Playlist da scaricare'}>
                                        <PlaylistSelect
                                            value={selectedPlaylist}
                                            options={playlistOptions}
                                            onChange={setSelectedPlaylist}
                                            disabled={busy || noPlaylists || link !== ''}
                                        />
                                    </Tooltip>
                                </Collapse>
                                <Tooltip label={foldersHint || (link ? 'Scarica il link inserito' : 'Scarica la playlist selezionata')}>
                                    <button
                                        className="accent with-icon"
                                        onClick={downloadPlaylist}
                                        disabled={busy || !canDownloadSource || !foldersOk}
                                    >
                                        <span className="btn-icon"><DownloadIcon /></span>
                                        Scarica
                                    </button>
                                </Tooltip>
                                {downloadErrors.length > 0 && (
                                    <Tooltip label={`${downloadErrors.length} download non riusciti: clicca per i dettagli`}>
                                        <button
                                            type="button"
                                            className="ghost small with-icon danger download-errors-btn"
                                            onClick={() => setShowDownloadErrors(true)}
                                            aria-label={`${downloadErrors.length} download non riusciti`}
                                        >
                                            <AlertIcon />
                                            {downloadErrors.length}
                                        </button>
                                    </Tooltip>
                                )}
                            </div>
                            </div>
                            {results ? (
                                <button className="accent with-icon" onClick={refresh} disabled={busy || !foldersOk}>
                                    <span className="btn-icon"><RefreshIcon /></span>
                                    Avvia nuova scansione
                                </button>
                            ) : (
                                <button className="accent with-icon" onClick={process} disabled={!canProcess}>
                                    <span className="btn-icon"><ConvertIcon /></span>
                                    Converti nomi e scrivi tag
                                </button>
                            )}
                            {busy && cancellable && (
                                <button className="danger-solid with-icon" onClick={cancelOp}>
                                    <span className="btn-icon"><CloseIcon /></span>
                                    Annulla
                                </button>
                            )}
                            <Tooltip label="Cancella tutti i tag ID3 dagli MP3 della cartella">
                                <button
                                    className="ghost with-icon danger"
                                    onClick={() => setConfirmClearTags(true)}
                                    disabled={!canClearTags}
                                >
                                    <span className="btn-icon"><TagOffIcon /></span>
                                    Cancella tag
                                </button>
                            </Tooltip>
                        </div>

                        {opProgress}
                    </div>
                </div>
                )}

                {/* Modalità semplificata: una sola card al centro, con la scelta
                    della playlist come protagonista. Niente cartelle (sono nelle
                    Impostazioni), anteprima o registro Attività: gli esiti
                    arrivano dai toast e dai risultati. Senza risultati la card è
                    centrata anche in verticale; dopo una conversione sale in cima
                    e lascia spazio alla tabella. */}
                {!showSettings && simpleMode && (
                    <div className={'simple-stage' + (results ? ' has-results' : '')}>
                        <section className="simple-hero fade-in">
                            <div className="simple-hero-badge" aria-hidden="true">
                                <DownloadIcon />
                            </div>
                            <h2 className="simple-hero-title">Scarica la tua musica</h2>
                            <p className="simple-hero-sub">
                                {noPlaylists
                                    ? 'Incolla il link di un video, oppure aggiungi una playlist nelle Impostazioni.'
                                    : 'Incolla il link di un video o scegli una playlist: i brani vengono scaricati e subito rinominati, con titolo e artista scritti nei tag.'}
                            </p>

                            {/* Link sopra playlist e azione, largo quanto loro: un
                                blocco unico, largo al massimo 526 px. Il link, se
                                inserito, ha la precedenza sulla playlist. */}
                            <div className="simple-hero-download">
                                <LinkField
                                    value={downloadLink}
                                    onChange={setDownloadLink}
                                    onSubmit={() => foldersOk && downloadPlaylist()}
                                    onAddToPlaylist={onAddLink}
                                    onPaste={isAndroid ? pasteLink : undefined}
                                    disabled={busy}
                                />

                                <div className="simple-hero-controls">
                                    {/* Select + "+" restano affiancati anche quando, su
                                        schermi stretti, i comandi si impilano. Il "+"
                                        compare solo finché non c'è nessuna playlist. Con un link
                                        inserito si chiudono (animati) e resta solo l'azione. */}
                                    <Collapse className="simple-hero-pick" collapsed={link !== ''}>
                                        <PlaylistSelect
                                            value={selectedPlaylist}
                                            options={playlistOptions}
                                            onChange={setSelectedPlaylist}
                                            disabled={busy || link !== ''}
                                        />
                                        {noPlaylists && (
                                        <Tooltip label="Aggiungi una playlist nelle Impostazioni">
                                            <button
                                                className="ghost simple-hero-add"
                                                onClick={() => openSettings('download')}
                                                disabled={busy || link !== ''}
                                                aria-label="Aggiungi una playlist nelle Impostazioni"
                                            >
                                                <PlusIcon />
                                            </button>
                                        </Tooltip>
                                        )}
                                    </Collapse>
                                    {/* Durante l'operazione il pulsante principale
                                        diventa "Annulla": una sola azione alla volta. */}
                                    {busy && cancellable ? (
                                        <button className="danger-solid simple-hero-action" onClick={cancelOp}>
                                            <CloseIcon />
                                            Annulla
                                        </button>
                                    ) : (
                                        <Tooltip label={!folder ? 'Scegli prima la cartella di partenza nelle Impostazioni' : foldersHint}>
                                            <button
                                                className="accent simple-hero-action"
                                                onClick={downloadPlaylist}
                                                disabled={busy || !canDownloadSource || !foldersOk}
                                            >
                                                <DownloadIcon />
                                                Scarica e converti
                                            </button>
                                        </Tooltip>
                                    )}
                                </div>
                            </div>

                            {opProgress}

                            {downloadErrors.length > 0 && (
                                <button
                                    type="button"
                                    className="ghost small danger simple-hero-errors"
                                    onClick={() => setShowDownloadErrors(true)}
                                >
                                    <AlertIcon />
                                    {downloadErrors.length === 1
                                        ? '1 download non riuscito'
                                        : `${downloadErrors.length} download non riusciti`}
                                </button>
                            )}

                            {/* Dove finiscono i brani, con la scorciatoia per cambiarlo. */}
                            <div className="simple-hero-foot">
                                <FolderLines
                                    folder={folder}
                                    destSameAsSource={destSameAsSource}
                                    destFolder={destFolder}
                                    folderMissing={folderMissing}
                                    destMissing={destMissing}
                                    onMissingClick={() => openSettings('general')}
                                    disabled={busy}
                                />
                            </div>
                        </section>
                    </div>
                )}

                {showSettings && draft && (
                    <div className="settings-layout">
                        <div className="settings-panel">
                            {settingsTab === 'general' && (
                                <section className="settings">
                                    <CheckOption
                                        label="Modalità semplificata"
                                        info="Quando attiva, la schermata principale mostra solo il download (da un link o da una playlist): «Scarica e converti» scarica i brani e li converte subito (nomi e tag), senza anteprima."
                                        checked={!!draft.simpleMode}
                                        onChange={(checked) => setDraft({ ...draft, simpleMode: checked } as rules.Config)}
                                        disabled={busy}
                                    />
                                    <CheckOption
                                        label="Cerca titolo e artista su MusicBrainz"
                                        info="Quando dal nome di una traccia non si capiscono titolo e artista (o la segni da rivedere), il popup propone il nome trovato su musicbrainz.org al posto di quello attuale. Per la ricerca il nome della traccia viene inviato a MusicBrainz."
                                        checked={!!draft.musicBrainz}
                                        onChange={(checked) => setDraft({ ...draft, musicBrainz: checked } as rules.Config)}
                                        disabled={busy}
                                    />
                                    <hr className="settings-divider" />
                                    <div className="settings-folders">{folderSettings}</div>
                                </section>
                            )}

                            {settingsTab === 'download' && (
                                <section className="settings">
                                    {/* Su Android yt-dlp è integrato nell'app: niente scelta
                                        tra copia gestita e percorso personalizzato. */}
                                    {!isAndroid && (
                                    <CheckOption
                                        className="ytdlp-toggle"
                                        label="Gestisci autonomamente yt-dlp"
                                        info="Quando attivo, l'app tiene una propria copia di yt-dlp in %AppData%\RenameMusic (scrivibile senza permessi di amministratore) e la aggiorna da sola. Se manca, puoi scaricarla dal tasto accanto a «Non presente» oppure, dopo una conferma, al primo download di una playlist. Quando disattivo, indichi a mano il percorso di una tua copia di yt-dlp."
                                        checked={ytDlpManaged}
                                        onChange={toggleYtDlpManaged}
                                        disabled={busy || ytDlpChecking}
                                    />
                                    )}

                                    <div className="ytdlp-panel">
                                        <div className="ytdlp-head">
                                            {/* Nome, stato (la versione, se presente) e in gestione
                                                autonoma il percorso della copia dell'app: se non
                                                c'è spazio il percorso va a capo. Il tasto per
                                                scaricarlo resta sempre subito a destra di "Non
                                                presente". */}
                                            <div className="ytdlp-head-info">
                                            <span className="ytdlp-title">yt-dlp</span>
                                            {ytDlpChecking ? (
                                                <span className="ytdlp-checking" role="status">
                                                    <span className="spinner" aria-hidden="true" />
                                                    Ricerca di una copia locale…
                                                </span>
                                            ) : state?.ytDlpAvailable ? (
                                                <span
                                                    className="ytdlp-badge ytdlp-ok"
                                                    title={state?.ytDlpVersion ? 'Versione di yt-dlp in uso' : undefined}
                                                >
                                                    {state?.ytDlpVersion || 'Presente'}
                                                </span>
                                            ) : (
                                                <span className="ytdlp-status">
                                                    <span className="ytdlp-badge ytdlp-missing">
                                                        {isAndroid ? 'Non ancora pronto' : 'Non presente'}
                                                    </span>
                                                    {!isAndroid && (
                                                        <Tooltip label="Scarica yt-dlp">
                                                            <button
                                                                className="ghost small ytdlp-install"
                                                                onClick={() => setConfirmDownloadYtDlp(true)}
                                                                disabled={busy}
                                                                aria-label="Scarica yt-dlp"
                                                            >
                                                                <DownloadIcon />
                                                            </button>
                                                        </Tooltip>
                                                    )}
                                                </span>
                                            )}
                                            {ytDlpManaged && !ytDlpChecking && state?.ytDlpEffectivePath && (
                                                <span className="ytdlp-location">
                                                    {!state?.ytDlpAvailable && (
                                                        <>
                                                            <span className="ytdlp-location-label">Verrà scaricato in:</span>{' '}
                                                        </>
                                                    )}
                                                    <code className="ytdlp-path">{state.ytDlpEffectivePath}</code>
                                                </span>
                                            )}
                                            {/* Fuori dalla gestione autonoma il percorso si imposta
                                                a mano, sulla stessa riga (scende sotto se non c'è
                                                spazio). */}
                                            {!ytDlpManaged && (
                                                <div className="ytdlp-path-edit">
                                                    <span className="ytdlp-location-label">Percorso:</span>
                                                    <input
                                                        type="text"
                                                        placeholder="Percorso a yt-dlp.exe"
                                                        value={ytDlpPathDraft}
                                                        onChange={(e) => setYtDlpPathDraft(e.target.value)}
                                                        onBlur={applyYtDlpPath}
                                                        disabled={busy}
                                                    />
                                                    <button className="ghost with-icon" onClick={browseYtDlp} disabled={busy}>
                                                        <span className="btn-icon"><FolderOpenIcon /></span>
                                                        Sfoglia
                                                    </button>
                                                </div>
                                            )}
                                            </div>
                                            {ytDlpChecking ? null : isAndroid ? (
                                                <Tooltip label="Aggiorna yt-dlp all'ultima versione (YouTube cambia spesso: se i download falliscono, aggiornalo)">
                                                    <button
                                                        className="ghost small with-icon ytdlp-install"
                                                        onClick={() => setConfirmDownloadYtDlp(true)}
                                                        disabled={busy}
                                                    >
                                                        <span className="btn-icon"><RefreshIcon /></span>
                                                        Aggiorna
                                                    </button>
                                                </Tooltip>
                                            ) : state?.ytDlpAvailable && ytDlpManaged ? (
                                                <Tooltip label="Rimuovi yt-dlp (elimina la copia gestita dall'app)">
                                                    <button
                                                        className="ghost small danger ytdlp-uninstall"
                                                        onClick={() => setConfirmUninstallYtDlp(true)}
                                                        disabled={busy}
                                                        aria-label="Rimuovi yt-dlp"
                                                    >
                                                        <RemoveIcon />
                                                    </button>
                                                </Tooltip>
                                            ) : null}
                                        </div>

                                        {busy && installProgress?.tool === 'yt-dlp' && (
                                            <OpProgress
                                                className="ytdlp-progress"
                                                percent={installPercent(installProgress)}
                                                label={installLabel(installProgress)}
                                            />
                                        )}

                                        {/* ffmpeg serve a yt-dlp per creare gli mp3. Su Android
                                            è incorporato in youtubedl-android. */}
                                        {!isAndroid && (
                                            <div className="ytdlp-head">
                                                <span className="ytdlp-title">ffmpeg</span>
                                                {state?.ffmpegAvailable ? (
                                                    <>
                                                        {/* Si rimuove solo la copia gestita: un
                                                            ffmpeg di sistema non è dell'app. */}
                                                        <span className="ytdlp-badge ytdlp-ok">
                                                            Presente{!state.ffmpegManaged && ' · di sistema'}
                                                        </span>
                                                        {state.ffmpegManaged && (
                                                            <Tooltip label="Rimuovi ffmpeg (elimina la copia gestita dall'app)">
                                                                <button
                                                                    className="ghost small danger ytdlp-uninstall"
                                                                    onClick={() => setConfirmUninstallFFmpeg(true)}
                                                                    disabled={busy}
                                                                    aria-label="Rimuovi ffmpeg"
                                                                >
                                                                    <RemoveIcon />
                                                                </button>
                                                            </Tooltip>
                                                        )}
                                                    </>
                                                ) : (
                                                    <>
                                                        <span className="ytdlp-badge ytdlp-missing">Non presente</span>
                                                        <Tooltip label="Scarica ffmpeg (serve a yt-dlp per creare gli mp3)">
                                                            <button
                                                                className="ghost small ytdlp-install"
                                                                onClick={() => setConfirmFFmpeg('install')}
                                                                disabled={busy}
                                                                aria-label="Scarica ffmpeg"
                                                            >
                                                                <DownloadIcon />
                                                            </button>
                                                        </Tooltip>
                                                    </>
                                                )}
                                            </div>
                                        )}

                                        {busy && installProgress?.tool === 'ffmpeg' && (
                                            <OpProgress
                                                className="ytdlp-progress"
                                                percent={installPercent(installProgress)}
                                                label={installLabel(installProgress)}
                                            />
                                        )}
                                    </div>

                                    {/* Account Google: serve a leggere e modificare le playlist
                                        di YouTube dell'utente. Senza le credenziali OAuth (solo
                                        desktop) resta visibile e spiega cosa manca. */}
                                        <div className="ytdlp-panel google-panel">
                                            {/* Titolo con la "i" a sinistra, stato (o email
                                                dell'account) allineato a destra accanto
                                                all'azione; su Android lo stato va sotto il
                                                titolo e l'azione resta a destra. */}
                                            <div className="ytdlp-head">
                                                <div className="ytdlp-head-info">
                                                    <span className="google-title">
                                                        <AccountIcon />
                                                        Account Google
                                                    </span>
                                                    <InfoIcon text="Collegando il tuo account l'app può leggere le tue playlist di YouTube (anche quelle private) per importarle qui, mostrarle per prime nella scelta del download, aggiungere a una playlist il video del link e, secondo le impostazioni di ogni playlist (⚙), togliere da YouTube i brani appena scaricati o aggiungerli a un'altra playlist. Puoi revocare l'accesso in qualsiasi momento con «Scollega»." />
                                                </div>
                                                {!googleAvailable ? (
                                                    <span className="ytdlp-badge google-off">Non configurato</span>
                                                ) : googleConnected ? (
                                                    <span className="google-account-row">
                                                        <span className="ytdlp-badge ytdlp-ok google-account" title="Account collegato">
                                                            {state?.google?.email || 'Collegato'}
                                                        </span>
                                                    </span>
                                                ) : (
                                                    <span className="ytdlp-badge google-off">Non collegato</span>
                                                )}
                                                {!googleAvailable ? null : googleConnected ? (
                                                    <button
                                                        className="ghost small danger"
                                                        onClick={() => setConfirmGoogleSignOut(true)}
                                                        disabled={busy}
                                                    >
                                                        Scollega
                                                    </button>
                                                ) : googleSigningIn && !isAndroid ? (
                                                    <button className="danger-solid small with-icon" onClick={cancelOp}>
                                                        <span className="btn-icon"><CloseIcon /></span>
                                                        Annulla
                                                    </button>
                                                ) : (
                                                    <button className="accent small with-icon" onClick={googleSignIn} disabled={busy}>
                                                        <span className="btn-icon"><AccountIcon /></span>
                                                        Collega
                                                    </button>
                                                )}
                                            </div>
                                            {googleConnected && !state?.google?.email && (
                                                <p className="google-hint google-sub">
                                                    Per vedere l'indirizzo dell'account collegato, scollegalo e ricollegalo.
                                                </p>
                                            )}
                                            {/* Impostazioni delle playlist condivise con gli altri
                                                dispositivi dello stesso account (Google Drive). */}
                                            {googleConnected && (
                                                prefsSyncing ? (
                                                    <p className="google-hint" role="status">
                                                        <span className="spinner" aria-hidden="true" />
                                                        Controllo delle impostazioni salvate sull'account…
                                                    </p>
                                                ) : (
                                                    <p className={'google-hint google-sub' + (state?.google?.syncError ? ' google-sync-error' : '')}>
                                                        {state?.google?.syncError ||
                                                            'Regole, playlist, predefiniti e impostazioni delle playlist (⚙) sono condivisi con gli altri dispositivi collegati a questo account. Cartelle e opzioni di conversione restano di ogni dispositivo.'}
                                                    </p>
                                                )
                                            )}
                                            {!googleAvailable && (
                                                <p className="google-hint">
                                                    Questa versione dell'app non ha le credenziali OAuth di Google
                                                    Cloud (internal/google/credentials_local.go): ricompilala con quelle
                                                    del client «App desktop» per collegare l'account.
                                                </p>
                                            )}
                                            {googleSigningIn && !isAndroid && (
                                                <p className="google-hint" role="status">
                                                    <span className="spinner" aria-hidden="true" />
                                                    Completa l'accesso nella pagina di Google che si è aperta nel browser, poi torna qui.
                                                </p>
                                            )}
                                        </div>

                                    {/* Playlist dell'account: le prime della scelta del
                                        download, ognuna con le sue impostazioni (⚙). */}
                                    {googleConnected && (
                                        <div className="replacements">
                                            <div className="replacements-head">
                                                <span>Playlist del tuo account YouTube</span>
                                            </div>
                                            {accountPlaylists.length === 0 ? (
                                                <p className="rule-empty">Il tuo account non ha ancora playlist.</p>
                                            ) : (
                                                <ul className="account-playlists">
                                                    {accountPlaylists.map((p) => {
                                                        const key = 'yt:' + p.id
                                                        const summary = prefsSummary(playlistPrefs[key])
                                                        return (
                                                            <li key={p.id} className={'account-playlist' + (playlistPrefs[key]?.hidden ? ' is-hidden' : '')}>
                                                                <span className="account-playlist-text">
                                                                    <span className="account-playlist-title">{p.title || p.id}</span>
                                                                    <span className="account-playlist-meta">
                                                                        {p.count === 1 ? '1 video' : `${p.count} video`}
                                                                        {summary && <> · <span className="account-playlist-prefs">{summary}</span></>}
                                                                    </span>
                                                                </span>
                                                                {emptying === p.id ? (
                                                                    <button className="danger-solid small with-icon" onClick={cancelOp}>
                                                                        <span className="btn-icon"><CloseIcon /></span>
                                                                        Annulla
                                                                    </button>
                                                                ) : (
                                                                    <PrefsButton
                                                                        summary={summary}
                                                                        onClick={() => openPlaylistPrefs(key, p.title || p.id)}
                                                                        disabled={busy}
                                                                    />
                                                                )}
                                                                {emptying === p.id && progress && progress.total > 0 && (
                                                                    <OpProgress
                                                                        className="account-playlist-progress"
                                                                        percent={Math.round((progress.done / progress.total) * 100)}
                                                                        label={`Svuotamento · ${progress.done} / ${progress.total} brani tolti`}
                                                                    />
                                                                )}
                                                            </li>
                                                        )
                                                    })}
                                                </ul>
                                            )}
                                        </div>
                                    )}

                                    <div className="replacements">
                                        <div className="replacements-head saved-playlists-head">
                                            <span>Playlist salvate (nome → link)</span>
                                            <span className="replacements-head-actions">
                                                {googleConnected && (
                                                    <button
                                                        className="ghost small with-icon"
                                                        onClick={() => openGooglePicker('import')}
                                                        disabled={busy}
                                                    >
                                                        <span className="btn-icon"><AccountIcon /></span>
                                                        Importa dall'account
                                                    </button>
                                                )}
                                                <button className="ghost small add-replacement" onClick={addPlaylistDraft} disabled={busy}>
                                                    + Aggiungi
                                                </button>
                                            </span>
                                        </div>
                                        {/* Anche l'ultima riga si toglie con la ✕: senza playlist
                                            resta solo l'avviso, e "Aggiungi" ne crea una. */}
                                        {playlistDraft.length === 0 && <p className="rule-empty">Nessuna playlist salvata.</p>}
                                        {playlistDraft.map((p, i) => (
                                            <div className="replacement-row playlist-row" key={i}>
                                                <input
                                                    type="text"
                                                    placeholder="Nome"
                                                    value={p.name}
                                                    onChange={(e) => updatePlaylistDraft(i, 'name', e.target.value)}
                                                    disabled={busy}
                                                />
                                                <span className="arrow">→</span>
                                                <input
                                                    type="text"
                                                    placeholder="Link playlist"
                                                    value={p.url}
                                                    onChange={(e) => updatePlaylistDraft(i, 'url', e.target.value)}
                                                    disabled={busy}
                                                />
                                                {/* Impostazioni della playlist: si aprono solo per una
                                                    riga compilata (nome e link). */}
                                                <PrefsButton
                                                    summary={prefsSummary(playlistPrefs[playlistKeyOf(p.url, p.name)])}
                                                    onClick={() => openPlaylistPrefs(playlistKeyOf(p.url, p.name), p.name.trim())}
                                                    disabled={busy || p.name.trim() === '' || p.url.trim() === ''}
                                                />
                                                <button
                                                    className="ghost small danger"
                                                    onClick={() => removePlaylistDraft(i)}
                                                    disabled={busy}
                                                    aria-label="Rimuovi playlist"
                                                >
                                                    ✕
                                                </button>
                                            </div>
                                        ))}
                                    </div>
                                </section>
                            )}

                            {settingsTab === 'rules' && (
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
                                                onChange={(v) => updateDraftList('occurrenciesToRemove', v)}
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
                                                    onChange={(e) => updateFtAlias(e.target.value)}
                                                    disabled={busy}
                                                />
                                            </label>
                                            <ChipList
                                                label="Varianti da sostituire con l'alias"
                                                caption="Varianti da sostituire"
                                                values={draft.occurrenciesToReplaceWithFt ?? []}
                                                onChange={(v) => updateDraftList('occurrenciesToReplaceWithFt', v)}
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
                                                onChange={(v) => updateDraftList('artistExceptions', v)}
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
                                                onChange={(v) => updateDraftList('supportedExtensions', v)}
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
                                                <RuleGroup
                                                    key={g.scope}
                                                    title={g.label}
                                                    hint={g.hint}
                                                    count={rows.length}
                                                    tone="yellow"
                                                >
                                                    {rows.length === 0 && <p className="rule-empty">Nessuna sostituzione.</p>}
                                                    {rows.map(({ r, i }) => (
                                                        <div className="replacement-row" key={i}>
                                                            <input
                                                                type="text"
                                                                placeholder="Da"
                                                                value={r.from}
                                                                onChange={(e) => updateReplacement(i, 'from', e.target.value)}
                                                                disabled={busy}
                                                            />
                                                            <span className="arrow">→</span>
                                                            <input
                                                                type="text"
                                                                placeholder="A"
                                                                value={r.to}
                                                                onChange={(e) => updateReplacement(i, 'to', e.target.value)}
                                                                disabled={busy}
                                                            />
                                                            <button
                                                                className="ghost small danger"
                                                                onClick={() => removeReplacement(i)}
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
                                                            onClick={() => addReplacement(g.scope)}
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
                            )}

                            {settingsTab === 'info' && (
                                <section className="settings about">
                                    <h2>RenameMusic</h2>
                                    <p className="about-desc">
                                        RenameMusic scarica le tue playlist di YouTube direttamente in MP3 e mette in ordine la tua musica in un clic, con nomi puliti e tag di titolo e artista coerenti.
                                    </p>
                                    <div className="ytdlp-panel">
                                        <div className="ytdlp-head">
                                            <span className="ytdlp-title">Versione {state?.appVersion}</span>
                                            {state?.update ? (
                                                <>
                                                    <span className="ytdlp-badge update-badge">
                                                        Disponibile la versione {state.update.version}
                                                    </span>
                                                    <button
                                                        className="ghost small with-icon ytdlp-install"
                                                        onClick={() => state.update && setUpdatePopup(state.update)}
                                                        disabled={busy}
                                                    >
                                                        <span className="btn-icon"><DownloadIcon /></span>
                                                        Aggiorna
                                                    </button>
                                                </>
                                            ) : (
                                                <Tooltip label="Controlla subito su GitHub se è uscita una nuova versione (l'app lo fa comunque da sola quando è connessa a Internet)">
                                                    <button
                                                        className="ghost small with-icon ytdlp-install"
                                                        onClick={checkUpdate}
                                                        disabled={busy}
                                                    >
                                                        <span className="btn-icon"><RefreshIcon /></span>
                                                        Verifica aggiornamenti
                                                    </button>
                                                </Tooltip>
                                            )}
                                        </div>
                                    </div>

                                    <dl className="about-list">
                                        <dt>Autore</dt>
                                        <dd>Simone D'Alessandro</dd>
                                        <dt>Codice sorgente</dt>
                                        <dd>
                                            <a
                                                href={REPO_URL}
                                                onClick={(e) => {
                                                    e.preventDefault()
                                                    openURL(REPO_URL)
                                                }}
                                            >
                                                github.com/SimoxSpeed/RenameMusic
                                            </a>
                                        </dd>
                                    </dl>

                                    <hr className="settings-divider" />

                                    <div className="about-disclaimer">
                                        <h3>Esclusione di responsabilità</h3>
                                        <p>
                                            RenameMusic è distribuito così com'è, senza alcuna garanzia. Rinomina e
                                            sposta file e, se lo attivi, elimina gli originali: prima di usarlo su
                                            una raccolta a cui tieni, fanne una copia. L'autore non risponde di
                                            perdite di dati o di altri danni causati dall'uso dell'app.
                                        </p>
                                        <p>
                                            Scaricare da YouTube può violare i suoi Termini di servizio e il diritto
                                            d'autore: scarica solo contenuti di cui hai i diritti o che sono
                                            distribuiti liberamente. L'uso che ne fai è sotto la tua responsabilità.
                                        </p>
                                    </div>
                                </section>
                            )}
                        </div>
                    </div>
                )}

                {/* In modalità semplificata niente anteprima: il pannello compare
                    solo con i risultati dell'ultima conversione. */}
                {!showSettings && (!simpleMode || results) && (
                <section className={'panel preview-panel fade-in' + (simpleMode ? ' simple-results' : '')}>
                    <div className="panel-head">
                        <h2>
                            <span className="h2-icon">{results ? <ConvertIcon /> : <EyeIcon />}</span>
                            {results ? 'Risultato conversione' : 'Anteprima'}
                        </h2>
                            {!results && (
                                <div className="preview-tools">
                                    <Tooltip label="Mostra solo i file che subiranno una modifica, nel nome o nei tag, e quelli selezionati da rivedere. È solo una vista: l'elaborazione tratta comunque tutti i file.">
                                        <label className="toggle-changed">
                                            <input
                                                type="checkbox"
                                                checked={showOnlyChanged}
                                                onChange={(e) => setShowOnlyChanged(e.target.checked)}
                                                disabled={busy}
                                            />
                                            Solo da modificare
                                        </label>
                                    </Tooltip>
                                    <Tooltip label={foldersHint || 'Aggiorna la scansione della cartella'}>
                                        <button
                                            className="ghost small with-icon"
                                            onClick={refresh}
                                            disabled={busy || !foldersOk}
                                        >
                                            <span className="btn-icon"><RefreshIcon /></span>
                                            Aggiorna
                                        </button>
                                    </Tooltip>
                                </div>
                            )}
                        </div>
                        {results ? (
                            results.length === 0 ? (
                                <div className="empty">Nessun file elaborato.</div>
                            ) : (
                                <table>
                                    <thead>
                                        <tr>
                                            <th>Nome originale</th>
                                            <th>Nuovo nome</th>
                                            <th>Titolo</th>
                                            <th>Artista</th>
                                            <th>Esito</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {results.map((r, i) => {
                                            const src = splitName(r.oldName)
                                            const dst = splitName(r.newName)
                                            const renamed = !r.skipped && !r.failed && !r.canceled && src.base !== dst.base
                                            // Titolo/Artista scritti nei tag: mostrati solo per gli MP3
                                            // effettivamente elaborati (non saltati/annullati), come nell'anteprima.
                                            const showTags = r.mp3 && !r.skipped && !r.canceled
                                            const rowClass = r.failed
                                                ? 'failed'
                                                : r.canceled
                                                  ? 'skipped'
                                                  : r.skipped
                                                    ? 'skipped'
                                                    : renamed
                                                      ? 'changed'
                                                      : ''
                                            return (
                                                <tr key={i} className={rowClass}>
                                                    <td data-label="Nome originale">
                                                        {renamed ? <s className="old-name">{src.base}</s> : src.base}
                                                    </td>
                                                    <td data-label="Nuovo nome">{r.skipped || r.canceled ? '—' : dst.base}</td>
                                                    <td data-label="Titolo">
                                                        {showTags ? r.title : <span className="muted-dash">—</span>}
                                                    </td>
                                                    <td data-label="Artista">
                                                        {showTags ? r.artist : <span className="muted-dash">—</span>}
                                                    </td>
                                                    <td data-label="Esito">
                                                        {r.failed ? (
                                                            <ErrorLabel message={r.reason} />
                                                        ) : r.canceled ? (
                                                            <span className="badge badge-neutral">Annullato</span>
                                                        ) : r.skipped ? (
                                                            <span className="note">Saltato: {r.reason}</span>
                                                        ) : (
                                                            <div className="badges">
                                                                {renamed ? (
                                                                    <span className="badge badge-changed">Rinominato</span>
                                                                ) : (
                                                                    <span className="badge badge-neutral">Invariato</span>
                                                                )}
                                                                {r.tagged && (
                                                                    <span className="badge badge-tag">Taggato</span>
                                                                )}
                                                                <ExtChip ext={dst.ext} />
                                                            </div>
                                                        )}
                                                    </td>
                                                </tr>
                                            )
                                        })}
                                    </tbody>
                                </table>
                            )
                        ) : !booted ? (
                            <div className="empty">Caricamento…</div>
                        ) : !folder ? (
                            <div className="empty">Scegli una cartella di partenza nelle Impostazioni per vedere l'anteprima.</div>
                        ) : folderMissing ? (
                            <div className="empty">Cartella di partenza non trovata.</div>
                        ) : files.length === 0 ? (
                            <div className="empty">Nessun file MP3 nella cartella di partenza.</div>
                        ) : previewFiles.length === 0 ? (
                            <div className="empty">Nessun file da modificare.</div>
                        ) : (
                            <table className="preview-table">
                                <thead>
                                    <tr>
                                        <th className="cell-select">
                                            <input
                                                type="checkbox"
                                                aria-label="Seleziona tutti da rivedere"
                                                title="Seleziona tutti da rivedere"
                                                checked={allVisibleSelected}
                                                ref={(el) => {
                                                    if (el) el.indeterminate = someVisibleSelected && !allVisibleSelected
                                                }}
                                                onChange={(e) => toggleReviewAll(e.target.checked)}
                                                disabled={busy}
                                            />
                                        </th>
                                        <th className="cell-current">File attuale</th>
                                        <th>Anteprima nuovo nome</th>
                                        <th>Anteprima nuovo titolo</th>
                                        <th>Anteprima nuovo artista</th>
                                        <th>Stato</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {previewFiles.map((file, i) => {
                                        const src = splitName(file.name)
                                        const dst = splitName(file.preview)
                                        const nameChanged = src.base !== dst.base
                                        const titleChanged = file.mp3 && tagChanged(file.title, file.titlePreview)
                                        const artistChanged = file.mp3 && tagChanged(file.artist, file.artistPreview)
                                        const rowChanged = nameChanged || titleChanged || artistChanged
                                        const toReview = reviewPaths.has(file.path)
                                        return (
                                            <tr key={i} className={(rowChanged ? 'changed' : '') + (toReview ? ' to-review' : '')}>
                                                <td className="cell-select">
                                                    <input
                                                        type="checkbox"
                                                        aria-label={'Rivedi ' + src.base}
                                                        checked={toReview}
                                                        onChange={(e) => toggleReview(file.path, e.target.checked)}
                                                        disabled={busy}
                                                    />
                                                </td>
                                                <td data-label="File attuale" className="cell-current">
                                                    <CurrentField label="nome" value={src.base} changed={nameChanged} />
                                                    {file.mp3 && (
                                                        <CurrentField label="titolo" value={file.title ?? ''} changed={titleChanged} />
                                                    )}
                                                    {file.mp3 && (
                                                        <CurrentField label="artista" value={file.artist ?? ''} changed={artistChanged} />
                                                    )}
                                                </td>
                                                <td data-label="Nuovo nome" className={nameChanged ? 'value-changed' : ''}>{dst.base}</td>
                                                <td data-label="Nuovo titolo" className={titleChanged ? 'value-changed' : ''}>
                                                    {file.mp3 ? file.titlePreview : <span className="muted-dash">—</span>}
                                                </td>
                                                <td data-label="Nuovo artista" className={artistChanged ? 'value-changed' : ''}>
                                                    {file.mp3 ? file.artistPreview : <span className="muted-dash">—</span>}
                                                </td>
                                                <td data-label="Stato">
                                                    <div className="badges">
                                                        {nameChanged ? (
                                                            <span className="badge badge-changed">Da rinominare</span>
                                                        ) : (
                                                            <span className="badge badge-neutral">Invariato</span>
                                                        )}
                                                        {(titleChanged || artistChanged) && (
                                                            <span className="badge badge-tag">Da taggare</span>
                                                        )}
                                                        {toReview && <span className="badge badge-review">Da rivedere</span>}
                                                        <ExtChip ext={src.ext} />
                                                    </div>
                                                </td>
                                            </tr>
                                        )
                                    })}
                                </tbody>
                            </table>
                        )}
                    </section>
                )}
            </main>
            </div>

            {folderPicker && (
                <FolderPicker
                    title={folderPicker === 'dest' ? 'Cartella di destinazione' : 'Cartella di partenza'}
                    initialPath={folderPicker === 'dest' ? destFolder || folder : folder}
                    onCancel={() => setFolderPicker(null)}
                    onSelect={pickFolder}
                />
            )}

            {tagPrompts.length > 0 && (
                <TagPromptDialog
                    prompts={tagPrompts}
                    search={promptSearch}
                    draft={promptDraft}
                    onDraftChange={setPromptDraft}
                    filter={promptFilter}
                    onFilterChange={setPromptFilter}
                    onSkipSearch={skipPromptSearch}
                    onResolve={resolvePrompt}
                />
            )}

            {showDownloadErrors && (
                <DownloadErrorsDialog errors={downloadErrors} onClose={() => setShowDownloadErrors(false)} />
            )}

            {/* Playlist dell'account Google: da spuntare per importarle nelle
                Impostazioni, oppure da scegliere per aggiungerci il video del link. */}
            {googlePicker && (
                <GooglePickerDialog
                    mode={googlePicker}
                    lists={googleLists}
                    savedIds={savedPlaylistIds}
                    selected={googleImport}
                    setSelected={setGoogleImport}
                    onClose={closeGooglePicker}
                    onImport={importGooglePlaylists}
                    onAdd={addLinkToPlaylist}
                />
            )}

            {/* Impostazioni di una playlist della scelta del download. */}
            {prefsTarget && (
                <PlaylistPrefsDialog
                    target={prefsTarget}
                    connected={googleConnected}
                    accountPlaylists={accountPlaylists}
                    draft={prefsDraft}
                    setDraft={setPrefsDraft}
                    busy={busy}
                    onClose={() => setPrefsTarget(null)}
                    onSave={savePlaylistPrefs}
                    onEmpty={() => setConfirmEmpty({ id: prefsTarget.ownId, title: prefsTarget.title })}
                />
            )}

            {confirmEmpty && (
                <EmptyPlaylistConfirm
                    title={confirmEmpty.title}
                    onCancel={() => setConfirmEmpty(null)}
                    onConfirm={emptyPlaylist}
                    disabled={busy}
                />
            )}

            {confirmGoogleSignOut && (
                <GoogleSignOutConfirm
                    onCancel={() => setConfirmGoogleSignOut(false)}
                    onConfirm={googleSignOut}
                    disabled={busy}
                />
            )}

            {confirmDeleteOriginals && (
                <DeleteOriginalsConfirm
                    onCancel={() => setConfirmDeleteOriginals(false)}
                    onConfirm={confirmEnableDelete}
                    disabled={busy}
                />
            )}

            {confirmClearTags && (
                <ClearTagsConfirm
                    onCancel={() => setConfirmClearTags(false)}
                    onConfirm={confirmClearTagsAction}
                    disabled={busy}
                />
            )}

            {confirmInstallYtDlp && (
                <InstallYtDlpConfirm
                    managed={ytDlpManaged}
                    ffmpegAvailable={!!state?.ffmpegAvailable}
                    downloadWhat={downloadWhat}
                    onCancel={() => setConfirmInstallYtDlp(false)}
                    onConfirm={confirmInstallThenDownload}
                    disabled={busy}
                />
            )}

            {confirmUninstallYtDlp && (
                <UninstallYtDlpConfirm
                    onCancel={() => setConfirmUninstallYtDlp(false)}
                    onConfirm={uninstallYtDlp}
                    disabled={busy}
                />
            )}

            {crashReport && <CrashDialog onShare={shareCrash} onDiscard={discardCrash} />}

            {confirmUninstallFFmpeg && (
                <UninstallFFmpegConfirm
                    onCancel={() => setConfirmUninstallFFmpeg(false)}
                    onConfirm={uninstallFFmpeg}
                    disabled={busy}
                />
            )}

            {confirmDownloadYtDlp && (
                <DownloadYtDlpConfirm
                    effectivePath={state?.ytDlpEffectivePath ?? ''}
                    managed={ytDlpManaged}
                    onCancel={() => setConfirmDownloadYtDlp(false)}
                    onConfirm={installYtDlp}
                    disabled={busy}
                />
            )}

            {confirmFFmpeg && (
                <FFmpegConfirm
                    reason={confirmFFmpeg}
                    downloadWhat={downloadWhat}
                    onCancel={() => setConfirmFFmpeg(null)}
                    onConfirm={installFFmpeg}
                    disabled={busy}
                />
            )}

            {confirmReset && (
                <ResetDefaultsConfirm
                    onCancel={() => setConfirmReset(false)}
                    onConfirm={confirmResetConfig}
                    disabled={busy}
                />
            )}

            {confirmDefault && (
                <SaveDefaultsConfirm
                    onCancel={() => setConfirmDefault(false)}
                    onConfirm={confirmMakeDefault}
                    disabled={busy}
                />
            )}

            {updatePopup && (
                <UpdateDialog
                    update={updatePopup}
                    appVersion={state.appVersion}
                    busy={busy}
                    installProgress={installProgress}
                    onClose={() => setUpdatePopup(null)}
                    onInstall={installUpdate}
                />
            )}

            <div
                className="toast-container"
                aria-live="polite"
                aria-atomic="false"
            >
                {toasts.map((t) => (
                    <div key={t.id} className={'toast ' + (t.ok ? 'toast-ok' : 'toast-err')} role="status">
                        <span className="toast-icon" aria-hidden="true">
                            {t.ok ? <CheckIcon /> : <AlertIcon />}
                        </span>
                        <span className="toast-msg">{t.message}</span>
                        <button
                            type="button"
                            className="toast-close"
                            aria-label="Chiudi notifica"
                            onClick={() => dismissToast(t.id)}
                        >
                            <CloseIcon />
                        </button>
                        {/* Barra del tempo residuo: si svuota in `duration` ms e
                            alla fine della sua animazione chiude il toast. */}
                        <span
                            className="toast-timer"
                            aria-hidden="true"
                            style={{ animationDuration: t.duration + 'ms' }}
                            onAnimationEnd={() => dismissToast(t.id)}
                        />
                    </div>
                ))}
            </div>
        </div>
    )
}

export default App
