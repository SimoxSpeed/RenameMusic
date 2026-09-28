// api.ts: unico punto di accesso della UI al core Go, identico su desktop e su
// Android. Su desktop (Wails) i metodi del core sono esposti da Wails in
// window.go.core.App; su Android (Capacitor) passano dal plugin nativo
// "RenameMusic", che li inoltra al core compilato con gomobile (package mobile)
// con argomenti e risultati in JSON. Gli eventi del core arrivano in entrambi
// i casi tramite onEvent.
import { Capacitor, registerPlugin, type PluginListenerHandle } from '@capacitor/core'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { core, playlist, rules } from '../wailsjs/go/models'

export type { core, playlist, rules }

// isAndroid: true quando la UI gira nell'app Android (WebView di Capacitor).
export const isAndroid = Capacitor.getPlatform() === 'android'

type EventPayload = { name: string; payload: string }

interface RenameMusicPlugin {
    call(options: { method: string; args: string }): Promise<{ result: string }>
    storageStatus(): Promise<{ granted: boolean }>
    requestStorage(): Promise<{ granted: boolean }>
    requestNotifications(): Promise<void>
    addListener(event: 'event', cb: (e: EventPayload) => void): Promise<PluginListenerHandle>
    addListener(event: 'resume', cb: (e: { storageGranted: boolean }) => void): Promise<PluginListenerHandle>
}

const native = registerPlugin<RenameMusicPlugin>('RenameMusic')

// call invoca un metodo del core per nome.
function call<T>(method: string, ...args: unknown[]): Promise<T> {
    if (isAndroid) {
        return native
            .call({ method, args: JSON.stringify(args) })
            .then((r) => JSON.parse(r.result) as T)
    }
    const bound = (window as any)?.go?.core?.App?.[method]
    if (typeof bound !== 'function') {
        return Promise.reject(new Error('Metodo non disponibile: ' + method))
    }
    return bound(...args) as Promise<T>
}

type Action = Promise<core.ActionResponse>

// ---- Metodi del core (stessi nomi e firme dei binding Wails) ----------------

export const GetState = (): Action => call('GetState')
export const GetConfig = (): Action => call('GetConfig')
export const SelectFolder = (): Action => call('SelectFolder')
export const SetFolder = (path: string): Action => call('SetFolder', path)
export const Scan = (): Action => call('Scan')
export const ProcessAll = (): Action => call('ProcessAll')
export const SetConfig = (cfg: rules.Config): Action => call('SetConfig', cfg)
export const ResetConfig = (): Action => call('ResetConfig')
export const SetAsDefault = (cfg: rules.Config, playlists: playlist.Playlist[]): Action =>
    call('SetAsDefault', cfg, playlists)
export const ClearLogs = (): Action => call('ClearLogs')
export const ChooseDirectory = (): Promise<string> => call('ChooseDirectory')
export const SetOptions = (sameAsSource: boolean, destination: string, deleteOriginals: boolean): Action =>
    call('SetOptions', sameAsSource, destination, deleteOriginals)
export const SetWatchEnabled = (enabled: boolean): Action => call('SetWatchEnabled', enabled)
export const OpenFolder = (path: string): Action => call('OpenFolder', path)
export const ClearTags = (): Action => call('ClearTags')
export const Cancel = (): Action => call('Cancel')
export const SetPlaylists = (list: playlist.Playlist[]): Action => call('SetPlaylists', list)
export const DownloadPlaylist = (name: string): Action => call('DownloadPlaylist', name)
export const DownloadAndProcess = (name: string): Action => call('DownloadAndProcess', name)
export const InstallYtDlp = (): Action => call('InstallYtDlp')
export const InstallFFmpeg = (): Action => call('InstallFFmpeg')
export const UninstallYtDlp = (): Action => call('UninstallYtDlp')
export const SetYtDlpConfig = (managed: boolean, path: string): Action => call('SetYtDlpConfig', managed, path)
export const ChooseYtDlpFile = (): Promise<string> => call('ChooseYtDlpFile')
export const ResolveTagPrompt = (path: string, useEdited: boolean, editedBase: string): Action =>
    call('ResolveTagPrompt', path, useEdited, editedBase)
export const CheckUpdate = (): Action => call('CheckUpdate')
export const MarkUpdateSeen = (version: string): Action => call('MarkUpdateSeen', version)
export const InstallUpdate = (): Action => call('InstallUpdate')

// ---- Solo Android ---------------------------------------------------------

// FolderListing: contenuto di una cartella per il selettore interno all'app
// (su Android il selettore di sistema non restituisce percorsi reali).
export type FolderRoot = { name: string; path: string }
export type FolderListing = {
    path: string
    parent: string
    dirs: string[]
    roots: FolderRoot[]
    error?: string
}

export const ListDirectory = (path: string): Promise<FolderListing> => call('ListDirectory', path)
export const MakeDirectory = (parent: string, name: string): Promise<FolderListing> =>
    call('MakeDirectory', parent, name)

// storageStatus / requestStorage: accesso a tutti i file (Android 11+), senza
// il quale il core non può leggere né rinominare i file musicali.
export async function storageStatus(): Promise<boolean> {
    if (!isAndroid) return true
    return (await native.storageStatus()).granted
}

export async function requestStorage(): Promise<boolean> {
    if (!isAndroid) return true
    return (await native.requestStorage()).granted
}

// requestNotifications chiede (una volta) il permesso per la notifica di
// avanzamento delle operazioni lunghe. Facoltativo: se negato tutto funziona.
export function requestNotifications(): void {
    if (isAndroid) native.requestNotifications().catch(() => {})
}

// onResume: l'app Android torna in primo piano (es. dalle impostazioni di sistema).
export function onResume(cb: (storageGranted: boolean) => void): () => void {
    if (!isAndroid) return () => {}
    const handle = native.addListener('resume', (e) => cb(e.storageGranted))
    return () => {
        handle.then((h) => h.remove())
    }
}

// ---- Eventi del core --------------------------------------------------------

type Handler = (payload: unknown) => void
const handlers = new Map<string, Set<Handler>>()
let nativeListener: Promise<PluginListenerHandle> | null = null

// onEvent registra un gestore per un evento del core (es. 'watch:changed') e
// restituisce la funzione per rimuoverlo.
export function onEvent(name: string, cb: Handler): () => void {
    if (!isAndroid) {
        return EventsOn(name, (payload: unknown) => cb(payload))
    }
    if (!nativeListener) {
        nativeListener = native.addListener('event', (e) => {
            const set = handlers.get(e.name)
            if (!set || set.size === 0) return
            let payload: unknown = null
            try {
                payload = e.payload ? JSON.parse(e.payload) : null
            } catch {
                return
            }
            set.forEach((h) => h(payload))
        })
    }
    let set = handlers.get(name)
    if (!set) {
        set = new Set()
        handlers.set(name, set)
    }
    set.add(cb)
    return () => {
        handlers.get(name)?.delete(cb)
    }
}
