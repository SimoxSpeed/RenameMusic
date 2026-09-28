# RenameMusic

App desktop (Wails v2) e **app Android** (APK, Capacitor) per **normalizzare i nomi dei file musicali** e scrivere i **tag ID3 degli MP3**. Porting in Go di un progetto Java originale. GUI in React + TypeScript, **la stessa** su entrambe le piattaforme; il core Go è condiviso.

L'utente sceglie una cartella di file audio; l'app mostra un'**anteprima** del nuovo nome per ciascun file (applicando una pipeline di regole configurabili), e su comando esplicito **rinomina/converte** i file scrivendo anche i tag MP3 (titolo/artista dedotti dal nome). Nessun file viene mai toccato senza il comando dell'utente.

## Stack e comandi

- **Backend**: Go 1.25 (`module renamemusic`). Dipendenze: `wails/v2 v2.13.0`, `fsnotify v1.10.1`.
- **Frontend**: React + TypeScript + Vite, in `frontend/` (build output in `frontend/dist`, embeddato nell'exe via `//go:embed all:frontend/dist` in [main.go](main.go)).
- **Binding Wails**: generati in `frontend/wailsjs/` (`go/core/App.*` per i metodi, `go/models.ts` per i tipi, namespace `core`/`rules`/`playlist`). **NON scrivere a mano**: rigenerare con `wails generate module` dopo aver cambiato firme/tipi dei metodi esportati di `core.App`. La UI **non importa** direttamente `wailsjs/go/...`: passa da [frontend/src/api.ts](frontend/src/api.ts) (vedi Android); dopo aver aggiunto/cambiato un metodo va aggiornato anche lì.
- **Android**: Capacitor 8 (`frontend/capacitor.config.ts`, progetto Gradle in `frontend/android/`), core Go compilato con `gomobile bind` (package [mobile/](mobile/)) in `frontend/android/app/libs/renamemusic.aar`, yt-dlp tramite `youtubedl-android` (Python + ffmpeg incorporati). `golang.org/x/mobile` è fissato a un commit compatibile con `go 1.25` (vedi `go.mod`): `gomobile`/`gobind` vanno installati alla stessa versione.

Toolchain sulla macchina (vedi memoria `build-toolchain`): Go in `D:\Programmi\bin` (nel PATH), Wails CLI e `gomobile`/`gobind` in `%USERPROFILE%\go\bin`, Node/npm nel PATH, Android SDK+NDK in `%LOCALAPPDATA%\Android\Sdk`, JDK 21 in `D:\Web_Programming\Java\jdk` (`JAVA_HOME` punta invece a un JDK 18).

- Build GUI: `wails build` → `build\bin\RenameMusic.exe`
- Dev hot-reload: `wails dev`
- Build APK: `powershell -ExecutionPolicy Bypass -File mobile\build-apk.ps1` → `build\bin\RenameMusic-arm64-v8a.apk` (telefono) e `-x86_64.apk` (emulatore). Cache/temporanei in `.android-build\` (ignorata da git).
- Installa APK sul dispositivo collegato (adb, sceglie l'ABI giusto): `powershell -ExecutionPolicy Bypass -File mobile\install-apk.ps1 [-Build] [-Serial <id>]`. Dalla radice o da `frontend/`: `npm run apk:build`, `npm run apk:install`, `npm run apk` (build + installazione).
- Release: `powershell -ExecutionPolicy Bypass -File release.ps1 patch|minor|major` (o `npm run release -- patch|minor|major`; l'argomento è **obbligatorio**, accettato anche come `-patch`/`--patch`): incrementa `update.Version`, compila exe + APK, e dopo conferma fa commit (messaggio fisso `chore(release): 🚀 release X.Y.Z`), tag `vX.Y.Z` e push (`--atomic`), poi apre la pagina della nuova release su GitHub. `-NoBuild` salta le build, `-Yes` la conferma. Lo lancia **l'utente** (vedi "Commit").
- Test: `go test ./...`
- Solo build frontend: `cd frontend; npm run build` (utile per verificare `tsc` + Vite senza Wails)
- `go build ./...` fallisce se `frontend/dist` è vuota (per via dell'embed) → `wails build` la popola.

> Il binario embedda `frontend/dist` a compile-time: modifiche al frontend si vedono solo dopo `wails build` (o in `wails dev`). Per l'APK le copia `npx cap sync` (lo fa lo script).

## Architettura

**`internal/core/app.go`** — il "core" applicativo (`core.App`), indipendente dalla piattaforma e condiviso da desktop e Android. Espone i metodi chiamati dalla UI e mantiene lo stato in memoria (cartella, regole correnti, ultimo scan, log, opzioni) sotto `sync.Mutex`. Tutte le risposte usano `ActionResponse { ok, message, state, results }`; `StateResponse` è lo snapshot completo che la UI assorbe (`absorb`). **Ogni metodo esportato di `core.App` è API della UI** (Wails lo binda, su Android è raggiungibile da `mobile.Call`): gli hook di ciclo di vita sono funzioni di package (`core.Start`, `core.HandleFileDrop`, `core.RefreshYtDlp`, `core.NotifyExternalChange`), non metodi.

Metodi principali: `GetState` (scansiona lazy la cartella ricordata al primo accesso, così la UI si popola in un colpo solo), `SelectFolder`/`SetFolder`, `Scan`, `ProcessAll(review)` (normalizza + scrive tag; i file spuntati nell'anteprima in `review` non si convertono subito ma tornano nei `prompts` con `review: true`, come le tracce con tag sconosciuti), `ChooseDirectory` (dialog destinazione), `SetOptions`, `SetConfig`/`ResetConfig`/`SetAsDefault`, `SetWatchEnabled`, `ClearLogs`, `DownloadPlaylist`, `DownloadAndProcess` (modalità semplificata: download + conversione in un'unica operazione annullabile), yt-dlp (`InstallYtDlp`, ...), aggiornamenti dell'app (`CheckUpdate`, `MarkUpdateSeen`, `InstallUpdate`, in [internal/core/update.go](internal/core/update.go)).

**`internal/core/platform.go`** — ciò che dipende dalla piattaforma passa dalle interfacce fornite a `core.New(core.Options{...})`: `Host` (eventi verso la UI, dialog di sistema, apertura cartella), `YtDlp` (dove/come gira yt-dlp; `ExecYtDlp` è quella desktop con l'eseguibile esterno) e `Updater` (installazione di una nuova versione dell'app; nil = nessun controllo aggiornamenti, come nei test).

**Desktop** — [main.go](main.go) (Wails, binda `*core.App`) + [host_desktop.go](host_desktop.go) (`Host` sopra il runtime Wails) + [update_desktop.go](update_desktop.go) (`Updater`: sostituisce l'exe e riavvia) + [open_windows.go](open_windows.go).

**Android** — [mobile/](mobile/) è il package per `gomobile bind`: `Start(dataDir, host)`, `Call(method, argsJSON)` (dispatch via reflection sui metodi di `core.App` + quelli di `folderAPI`, argomenti/risultati JSON: l'equivalente dei binding Wails), `Resume()`, `YtDlpInitialized()`. `Host` è implementata in Java: [frontend/android/app/src/main/java/com/renamemusic/app/](frontend/android/app/src/main/java/com/renamemusic/app/) — `RenameMusicPlugin` (plugin Capacitor: smista le chiamate su un pool di thread, permesso "accesso a tutti i file", servizio in primo piano durante le operazioni lunghe, riscansione MediaStore), `GoHost` (eventi + yt-dlp via youtubedl-android con gli stessi argomenti del desktop + installer degli aggiornamenti, `InstallApk`), `WorkService`, `MediaRescan`. `Updater` Android in [mobile/update.go](mobile/update.go).

**`internal/`**:
- **`rules`** — `Config` (regole configurabili: estensioni supportate, occorrenze da rimuovere, alias di "ft", sostituzioni From→To; più `SimpleMode`, vedi sotto) e `NormalizeFileBase()`, la **pipeline di normalizzazione** del nome (stesso ordine del Java originale: rimozione occorrenze → alias ft → `(ft` → sostituzioni → rimozione `[...]` → collapse spazi/trim → dash iniziale). `FactoryConfig()` è il seed di fabbrica.
- **`parser`** — estrazione di estensione, base name, e **titolo/artista per i tag** dal nome file (gestisce ` - `, ` ft `, remix, VIP, ecc.).
- **`fs`** — `ScanAudioFiles` (elenca i file audio supportati nella cartella, **non ricorsivo**), `IsDir`.
- **`rename`** — `Service.Process()`: calcola i nomi di destinazione, risolve le collisioni nel batch (un vincitore per nome), sposta/copia (`DeleteOriginals` on/off) e scrive i tag MP3. `Options { DestinationFolder, DeleteOriginals }`.
- **`tags`** — scrittura tag ID3 MP3.
- **`settings`** — persistenza JSON in `%AppData%\RenameMusic\` (su Android nella cartella privata dell'app, via `settings.SetDir`): `config.json` (regole correnti), `defaults.json` (default editabili), `state.json` (cartella, destinazione, elimina-originali, aggiornamento automatico). `Config` con campi mancanti eredita i valori di fabbrica.
- **`playlist`** — download di playlist con yt-dlp. `Runner` astrae l'esecuzione (`ExecRunner` su desktop, bridge verso youtubedl-android su Android): enumerazione, concorrenza ed estrazione errori sono comuni. Su desktop anche ffmpeg (necessario per l'mp3) ha una copia gestita in `%AppData%\RenameMusic\ffmpeg` (build di `yt-dlp/FFmpeg-Builds`, scaricata da `InstallFFmpeg` o insieme a yt-dlp se manca), passata a yt-dlp con `--ffmpeg-location`; senza di essa si usa l'ffmpeg del PATH. Con yt-dlp gestito il core lo **aggiorna da sé** ([internal/core/ytdlp_update.go](internal/core/ytdlp_update.go): all'avvio e ogni 24 ore, `YtDlp.Update`), mai durante un download (`ytDlpMu`): una versione vecchia di qualche settimana dà `HTTP Error 403`.
- **`watcher`** — wrapper `fsnotify` per l'**aggiornamento automatico** della cartella sorgente.
- **`update`** — aggiornamenti dell'app: `Version` (**unica fonte della versione**, letta anche da `build-apk.ps1` per `versionName`/`versionCode`), `Latest` (ultima release GitHub di `SimoxSpeed/RenameMusic`), `Newer`, `Download` (verifica dimensione e sha256), `ReplaceExecutable`/`CleanupExecutable` (desktop: rename dell'exe in uso su `.old`). `RENAMEMUSIC_UPDATE_URL` sostituisce l'URL dell'API per provare con un server locale.

**`frontend/src/App.tsx`** — unico componente principale. `guard()` avvolge le azioni async (imposta `busy`, gestisce errori, garantisce una **durata minima** del busy per non far lampeggiare la barra). Eventi del core (via `onEvent` di `api.ts`): `watch:changed` aggiorna solo l'anteprima, `process:progress`, `install:progress` (percentuale del download di yt-dlp/ffmpeg e dell'aggiornamento, `tool: "RenameMusic"`), `folder:dropped`, `ytdlp:changed`, `update:available`.

**`frontend/src/api.ts`** — unico accesso della UI al core: su desktop `window.go.core.App`, su Android il plugin `RenameMusic` (`isAndroid`). Le differenze di UI Android sono condizionate a `isAndroid` (desktop invariato): selettore cartelle interno ([FolderPicker.tsx](frontend/src/FolderPicker.tsx)), banner per il permesso sui file, tasto Indietro, yt-dlp integrato (solo "Aggiorna"), layout mobile in [mobile.css](frontend/src/mobile.css) (tutto sotto `.is-android`).

## Concetti chiave / invarianti

- **Regole correnti vs default**: le "correnti" (`config.json`) sono attive; i "default" (`defaults.json`) sono un preset ripristinabile. "Salva come predefinito" sovrascrive i default; "Ripristina default" copia i default nelle correnti. La cartella si gestisce a parte (`state.json`), mai dentro le regole.
- **Modalità semplificata** (`rules.Config.SimpleMode`, default `false`, anche nei predefiniti): preferenza di UI salvata con le regole per avere la stessa logica correnti/predefiniti. Se attiva, la schermata principale ha solo una card centrata (`.simple-hero`) con la scelta della playlist e "Scarica e converti" (`DownloadAndProcess`: stesso download e stessa conversione di `DownloadPlaylist` + `ProcessAll`, popup delle tracce da confermare compresi), senza anteprima (il pannello compare solo con i risultati) né registro Attività (gli esiti restano nei toast); cartelle e opzioni di conversione passano nella sezione "Generale" delle Impostazioni (si applicano subito, non col "Salva"), l'aggiornamento automatico è nascosto e il core ne ignora gli eventi. La schermata principale segue il valore salvato, le Impostazioni la bozza. `process:progress` porta `phase` (`download`/`convert`) per etichettare le due fasi.
- **Aggiornamento automatico** (ex "watch"): osserva la cartella e aggiorna l'**anteprima** quando cambia il contenuto; **non converte mai** automaticamente. Dopo un `ProcessAll` il watcher va in pausa (`watchPaused`) fino al prossimo Scan, per ignorare gli eventi fsnotify auto-generati. In tutte le label UI il termine è **"Aggiornamento automatico"** (o "Agg. automatico"), non "watch".
- **Formato file**: i file trattati sono **sempre e solo mp3** (nessuna conversione fra formati diversi). La UI mostra per riga un chip blu con l'estensione (`ExtChip`), perché **l'estensione non è mai mostrata nei nomi file**, solo nel chip. `rename` mantiene comunque l'estensione del file (sorgente = destinazione).
- **Android**: l'app è installata a mano (mai sul Play Store), quindi usa `MANAGE_EXTERNAL_STORAGE` e lavora su **percorsi reali** come il desktop (niente SAF/URI `content://`). Il core Go è compilato solo per `arm64-v8a` e `x86_64`: gli APK sono divisi per ABI (`splits` in `app/build.gradle`).
- **Aggiornamenti dell'app**: il core (se ha un `Updater`) controlla l'ultima release GitHub all'avvio, poi ogni 6 ore; se fallisce (offline) riprova ogni 2 minuti, e la UI ricontrolla subito all'evento `online`. Il popup "Nuova versione disponibile" compare **una sola volta per versione** (`MarkUpdateSeen` → `updateSeenVersion` in `state.json`); dopo resta il tasto "Aggiorna" nella sezione "Aggiornamenti" delle Impostazioni (e un pallino sul pulsante Impostazioni). Desktop: scarica `RenameMusic.exe` accanto all'exe in uso, lo sostituisce e riavvia. Android: scarica `RenameMusic-<abi>.apk` in `files/update/` e apre l'installer (FileProvider + `REQUEST_INSTALL_PACKAGES`; se manca il permesso "installa app sconosciute" apre le impostazioni e installa al ritorno). L'APK nuovo deve avere la **stessa firma** (keystore di debug della macchina di build) e un `versionCode` maggiore.
- **Pubblicare una versione**: `release.ps1` (incrementa `update.Version`, `wails build` + `mobile\build-apk.ps1`, commit + tag `v<Version>` + push), poi creare su GitHub la release su quel tag (non pre-release) allegando `build\bin\RenameMusic.exe`, `RenameMusic-arm64-v8a.apk` e `RenameMusic-x86_64.apk` con **esattamente questi nomi**. Il testo della release appare nel popup.
- **Log strutturati**: le righe di attività sono `LogEntry { time, kind, message }` con `LogKind` = `info | success | error | auto`, assegnato **alla sorgente** in `addLogLocked(kind, message)`. Il frontend le rende direttamente (niente parsing/euristiche sul testo). Max 12 righe, più recenti in cima.

## Convenzioni

- Commenti e stringhe UI/log **in italiano**.
- Riferimenti a file clickabili come `path:line`.
- `docs/` è locale e ignorata da git (`.gitignore`).

### Commit

- Si usano i **Conventional Commits** con gitmoji, con il **subject in inglese** e **senza corpo/descrizione** (es. `fix: 🐛 use atomic processes to write files & settings`), coerente con lo storico del repo.
- Claude **non esegue mai commit** né staging finalizzato a un commit: al massimo **propone** il messaggio quando l'utente lo richiede. **Committa sempre l'utente.**
