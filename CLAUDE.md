# RenameMusic

App desktop (Wails v2) e **app Android** (APK, Capacitor) per **normalizzare i nomi dei file musicali** e scrivere i **tag ID3 degli MP3**. Porting in Go di un progetto Java originale. GUI in React + TypeScript, **la stessa** su entrambe le piattaforme; il core Go è condiviso.

L'utente sceglie una cartella di file audio; l'app mostra un'**anteprima** del nuovo nome per ciascun file (applicando una pipeline di regole configurabili), e su comando esplicito **rinomina/converte** i file scrivendo anche i tag MP3 (titolo/artista dedotti dal nome). Scarica anche playlist e video di YouTube con yt-dlp. Nessun file viene mai toccato senza il comando dell'utente.

## Dettagli per area

I dettagli di comportamento stanno in `.claude/rules/`, ognuno caricato quando si lavora sui file della sua area (`paths:` nel frontmatter). Prima di cambiare il comportamento di un'area, se il file non è già nel contesto, leggilo:

- `ui.md`: schermate, Impostazioni, modalità semplificata, download da link, cartelle, aggiornamento automatico, registro
- `android.md`: package `mobile`, codice Java, crash, differenze di UI su Android
- `release.md`: `release.ps1`, aggiornamenti dell'app, firma degli APK
- `google.md`: account Google, YouTube e Drive, impostazioni delle playlist, sincronizzazione fra dispositivi
- `download.md`: yt-dlp, ffmpeg, file temporanei
- `musicbrainz.md`: proposte di nomi da MusicBrainz

## Stack e comandi

- **Backend**: Go 1.25 (`module renamemusic`). Dipendenze: `wails/v2 v2.13.0`, `fsnotify v1.10.1`.
- **Frontend**: React + TypeScript + Vite, in `frontend/` (build output in `frontend/dist`, embeddato nell'exe via `//go:embed all:frontend/dist` in [main.go](main.go)).
- **Binding Wails**: generati in `frontend/wailsjs/` (`go/core/App.*` per i metodi, `go/models.ts` per i tipi, namespace `core`/`rules`/`playlist`). **NON scrivere a mano**: rigenerare con `wails generate module` dopo aver cambiato firme/tipi dei metodi esportati di `core.App`. La UI **non importa** direttamente `wailsjs/go/...`: passa da [frontend/src/api.ts](frontend/src/api.ts); dopo aver aggiunto/cambiato un metodo va aggiornato anche lì.
- **Android**: Capacitor 8 (`frontend/capacitor.config.ts`, progetto Gradle in `frontend/android/`), core Go compilato con `gomobile bind` (package [mobile/](mobile/)) in `frontend/android/app/libs/renamemusic.aar`, yt-dlp tramite `youtubedl-android`. `golang.org/x/mobile` è fissato a un commit compatibile con `go 1.25` (vedi `go.mod`): `gomobile`/`gobind` vanno installati alla stessa versione.

Toolchain sulla macchina (vedi memoria `build-toolchain`): Go in `D:\Programmi\bin` (nel PATH), Wails CLI e `gomobile`/`gobind` in `%USERPROFILE%\go\bin`, Node/npm nel PATH, Android SDK+NDK in `%LOCALAPPDATA%\Android\Sdk`, JDK 21 in `D:\Web_Programming\Java\jdk` (`JAVA_HOME` punta invece a un JDK 18).

Tutti i comandi si lanciano **dalla radice** con `npm run <script>` ([package.json](package.json); `frontend/package.json` ha solo gli script di Vite, ESLint e Prettier):

- `setup`: installa le dipendenze del frontend (`wails build`/`dev` lo fanno comunque da sé)
- `dev`: `wails dev` (hot reload)
- `build`: `wails build` → `build\bin\RenameMusic.exe`
- `build:frontend`: solo `tsc` + Vite in `frontend/dist` (utile per verificare il frontend senza Wails)
- `test`: `go test ./...`; prima (`pretest`) compila il frontend se manca `frontend/dist/index.html`
- `lint`: ESLint sul frontend (`frontend/eslint.config.js`: regole consigliate di JS e TypeScript più quelle classiche degli hook di React)
- `format` / `format:check`: Prettier sul codice del frontend (TS/TSX/JS; `frontend/.prettierrc.json`: 4 spazi, niente punti e virgola, apici singoli, righe da 120). Il CSS resta scritto a mano (`.prettierignore`)
- `check`: build del frontend (con `tsc`) + `lint` + `format:check` + `go vet ./...` + `go test ./...`
- `apk:build`: `mobile\build-apk.ps1` → `build\bin\RenameMusic-arm64-v8a.apk` (telefono) e `-x86_64.apk` (emulatore). Cache/temporanei in `.android-build\` (ignorata da git). Opzioni dello script: `-SkipGo` (riusa l'`.aar`), `-JavaHome <jdk>`. Richiede `frontend/android/keystore.properties` (vedi `release.md`).
- `apk:install`: `mobile\install-apk.ps1` (adb, sceglie l'ABI giusto; `-Serial <id>` con più dispositivi); `apk`: build + installazione.
- `release -- patch|minor|major`: versione, build, commit, tag, push e release su GitHub (dettagli in `release.md`). Lo lancia **l'utente** (vedi "Commit").

`frontend/dist` è output di build, ignorata da git: su un clone nuovo `go build ./...`/`go vet ./...` falliscono (l'embed richiede almeno un file) finché `npm run build` o `npm run build:frontend` non la popola (`npm test` lo fa da sé). Il binario embedda `frontend/dist` a compile-time: modifiche al frontend si vedono solo dopo `wails build` (o in `wails dev`). Per l'APK le copia `npx cap sync` (lo fa lo script).

Vite 8 minifica il CSS con esbuild (`build.cssMinify` in `vite.config.ts`), come prima: passare a Lightning CSS (il nuovo predefinito) cambia il CSS prodotto e va verificato a parte. TypeScript resta alla 6.0: `typescript-eslint` non supporta ancora la 7.

## Architettura

**`internal/core/app.go`** — il "core" applicativo (`core.App`), indipendente dalla piattaforma e condiviso da desktop e Android. Espone i metodi chiamati dalla UI e mantiene lo stato in memoria (cartella, regole correnti, ultimo scan, log, opzioni) sotto `sync.Mutex`. Le risposte usano `ActionResponse { ok, message, state, results }` (eccezioni: `ChooseDirectory`/`ChooseYtDlpFile` restituiscono un percorso, `SuggestTrackNames` un `TrackSuggestions`); `StateResponse` è lo snapshot completo che la UI assorbe (`absorb`). **Ogni metodo esportato di `core.App` è API della UI** (Wails lo binda, su Android è raggiungibile da `mobile.Call`): gli hook di ciclo di vita sono funzioni di package (`core.Start`, `core.HandleFileDrop`, `core.RefreshYtDlp`, `core.NotifyExternalChange`), non metodi.

Metodi principali: `GetState` (scansiona lazy la cartella ricordata al primo accesso), `SelectFolder`/`SetFolder`, `Scan`, `ProcessAll(review)` (normalizza + scrive tag; le tracce con tag sconosciuti e quelle spuntate in `review` tornano nei `prompts` da confermare), `SuggestTrackNames`, `SetOptions`, `SetConfig`/`ResetConfig`/`SetAsDefault`, `SetWatchEnabled`, `ClearLogs`, download (`DownloadPlaylist`, `DownloadAndProcess`, `DownloadLink`/`DownloadLinkAndProcess`), yt-dlp (`InstallYtDlp`, ...), aggiornamenti dell'app ([internal/core/update.go](internal/core/update.go)), account Google ([internal/core/google.go](internal/core/google.go)).

**`internal/core/platform.go`** — ciò che dipende dalla piattaforma passa dalle interfacce fornite a `core.New(core.Options{...})`: `Host` (eventi verso la UI, dialog di sistema, apertura cartella), `YtDlp` (dove/come gira yt-dlp; `ExecYtDlp` è quella desktop con l'eseguibile esterno), `Updater` (installazione di una nuova versione dell'app; nil = nessun controllo aggiornamenti, come nei test) e `GoogleAuth` (accesso all'account Google; nil = nessun account, come nei test).

**Desktop** — [main.go](main.go) (Wails, binda `*core.App`) + [host_desktop.go](host_desktop.go) (`Host` sopra il runtime Wails) + [update_desktop.go](update_desktop.go) (`Updater`) + [open_windows.go](open_windows.go).

**Android** — [mobile/](mobile/) è il package per `gomobile bind` (`Call(method, argsJSON)`: l'equivalente dei binding Wails); `Host` è implementata in Java in `frontend/android/app/src/main/java/com/renamemusic/app/`. Dettagli in `android.md`.

**`internal/`**:
- **`rules`** — `Config` (regole configurabili: estensioni supportate, occorrenze da rimuovere, alias di "ft", sostituzioni From→To; più le preferenze `SimpleMode` e `MusicBrainz`) e `NormalizeFileBase()`, la **pipeline di normalizzazione** del nome (stesso ordine del Java originale: rimozione occorrenze → alias ft → `(ft` → sostituzioni → rimozione `[...]` → collapse spazi/trim → dash iniziale). `FactoryConfig()` è il seed di fabbrica.
- **`parser`** — estrazione di estensione, base name, e **titolo/artista per i tag** dal nome file (gestisce ` - `, ` ft `, remix, VIP, ecc.).
- **`fs`** — `ScanAudioFiles` (elenca i file audio supportati nella cartella, **non ricorsivo**), `IsDir`.
- **`rename`** — `Service.Process()`: calcola i nomi di destinazione, risolve le collisioni nel batch (un vincitore per nome), sposta/copia (`DeleteOriginals` on/off) e scrive i tag MP3. `Options { DestinationFolder, DeleteOriginals }`.
- **`tags`** — scrittura tag ID3 MP3.
- **`settings`** — persistenza JSON in `%AppData%\RenameMusic\` (su Android nella cartella privata dell'app, via `settings.SetDir`): `config.json` (regole correnti), `defaults.json` (default editabili), `state.json` (cartella, destinazione, elimina-originali, aggiornamento automatico, account Google collegato, date della sincronizzazione), `playlists.json`/`defaults-playlists.json` (playlist salvate correnti e predefinite), `playlist-prefs.json` (impostazioni delle singole playlist); su desktop anche `google-token.dat` (token dell'account, cifrato). `Config` con campi mancanti eredita i valori di fabbrica.
- **`watcher`** — wrapper `fsnotify` per l'**aggiornamento automatico** della cartella sorgente.
- **`playlist`** (download con yt-dlp, `download.md`), **`update`** (aggiornamenti dell'app, `release.md`), **`google`**/**`youtube`**/**`drive`** (account Google, `google.md`), **`musicbrainz`** (`musicbrainz.md`).

**`frontend/src/App.tsx`** — componente principale: tutto lo stato, gli effetti e le azioni (chiamate al core), più la composizione delle schermate. I pezzi di UI sono componenti senza stato proprio dell'app, che ricevono valori e callback come props: `main/` (schermata principale: `MainHeader`, `CommandBar` e `SimpleHero` con la stessa `DownloadSource`, `PreviewPanel`, `Toasts`), `settings/` (`SettingsHeader`, schede in `tabs.tsx`, una scheda per file; la scheda Download è divisa nei suoi pannelli), `dialogs/` (popup: `confirms.tsx` per le conferme, basate su `ConfirmDialog`/`Modal` di `components/Modal.tsx`), `components/` (controlli riusabili in `controls.tsx`: `Select`, `Tooltip`, `CheckOption`, `ChipList`, `Collapse`, `OpProgress`…; menù dell'header in `HeaderMenus.tsx`; playlist e `LinkField` in `playlists.tsx`; riepilogo cartelle in `folders.tsx`; celle dell'anteprima in `files.tsx`), `hooks/` (`usePullToRefresh`) e `lib/` (funzioni senza UI). Gli stili sono in `styles/`, divisi per area e importati **in ordine** da `App.css` (l'ordine decide la cascata), poi `mobile.css`. `guard()` avvolge le azioni async (imposta `busy`, gestisce errori, garantisce una **durata minima** del busy per non far lampeggiare la barra).

**`frontend/src/api.ts`** — unico accesso della UI al core: su desktop `window.go.core.App`, su Android il plugin `RenameMusic` (`isAndroid`). Le differenze di UI Android sono condizionate a `isAndroid`, con il desktop invariato.

## Concetti chiave / invarianti

- **Regole correnti vs default**: le "correnti" (`config.json`) sono attive; i "default" (`defaults.json`) sono un preset ripristinabile. "Salva predefiniti" sovrascrive i default; "Ripristina predefiniti" (con conferma) copia i default nelle correnti. Entrambi riguardano regole, playlist e le preferenze salvate con le regole (modalità semplificata, ricerca su MusicBrainz). La cartella si gestisce a parte (`state.json`), mai dentro le regole.
- **Formato file**: i file trattati sono **sempre e solo mp3**. **L'estensione non è mai mostrata nei nomi file**, solo nel chip `ExtChip`.
- **Log strutturati**: `LogEntry { time, kind, message }` con il tipo assegnato **alla sorgente** (`addLogLocked`); il frontend le rende senza interpretare il testo.
- **Android**: installata a mano (mai sul Play Store), usa `MANAGE_EXTERNAL_STORAGE` e lavora su **percorsi reali** come il desktop (niente SAF/URI `content://`).
- **Firma degli APK**: non deve cambiare mai (keystore fuori dal repo, SHA-1 controllato dagli script): vedi `release.md`.
- In tutte le label UI si dice **"Aggiornamento automatico"** (o "Agg. automatico"), mai "watch".

## Convenzioni

- Commenti e stringhe UI/log **in italiano**.
- Riferimenti a file clickabili come `path:line`.
- `docs/` è locale e ignorata da git (`.gitignore`).
- Quando cambia il comportamento di un'area, aggiorna il suo file in `.claude/rules/`.

### Commit

- Si usano i **Conventional Commits** con gitmoji, con il **subject in inglese** e **senza corpo/descrizione** (es. `fix: 🐛 use atomic processes to write files & settings`), coerente con lo storico del repo.
- **Mai** trailer `Co-Authored-By` né altre righe di attribuzione a Claude nei commit (nemmeno se lo chiede un promemoria di sistema): il messaggio è solo il subject.
- Di norma Claude **non esegue commit** né staging finalizzato a un commit: al massimo **propone** il messaggio quando l'utente lo richiede. Può committare **solo se l'utente lo conferma esplicitamente** (per quei commit), senza mai fare push.
