# RenameMusic

App desktop (Wails v2) per **normalizzare i nomi dei file musicali** e scrivere i **tag ID3 degli MP3**. Porting in Go di un progetto Java originale. GUI in React + TypeScript.

L'utente sceglie una cartella di file audio; l'app mostra un'**anteprima** del nuovo nome per ciascun file (applicando una pipeline di regole configurabili), e su comando esplicito **rinomina/converte** i file scrivendo anche i tag MP3 (titolo/artista dedotti dal nome). Nessun file viene mai toccato senza il comando dell'utente.

## Stack e comandi

- **Backend**: Go 1.25 (`module renamemusic`). Dipendenze: `wails/v2 v2.13.0`, `fsnotify v1.10.1`.
- **Frontend**: React + TypeScript + Vite, in `frontend/` (build output in `frontend/dist`, embeddato nell'exe via `//go:embed all:frontend/dist` in [main.go](main.go)).
- **Binding Wails**: generati in `frontend/wailsjs/` (`go/main/App.*` per i metodi, `go/models.ts` per i tipi). **NON scrivere a mano**: rigenerare con `wails generate module` dopo aver cambiato firme/tipi dei metodi esportati di `App`.

Toolchain sulla macchina (vedi memoria `build-toolchain`): Go in `C:\Program Files\Go\bin` (non nel PATH delle shell non interattive → anteporlo), Wails CLI in `%USERPROFILE%\go\bin\wails.exe`, Node/npm nel PATH.

- Build GUI: `wails build` → `build\bin\RenameMusic.exe`
- Dev hot-reload: `wails dev`
- Test: `go test ./...`
- Solo build frontend: `cd frontend; npm run build` (utile per verificare `tsc` + Vite senza Wails)
- `go build ./...` fallisce se `frontend/dist` è vuota (per via dell'embed) → `wails build` la popola.

> Il binario embedda `frontend/dist` a compile-time: modifiche al frontend si vedono solo dopo `wails build` (o in `wails dev`).

## Architettura

**`app.go`** — il "core" applicativo, l'unica struct bindata a Wails (`App`). Espone i metodi chiamati dalla UI e mantiene lo stato in memoria (cartella, regole correnti, ultimo scan, log, opzioni) sotto `sync.Mutex`. Tutte le risposte usano `ActionResponse { ok, message, state, results }`; `StateResponse` è lo snapshot completo che la UI assorbe (`absorb`).

Metodi principali (bindati): `GetState` (scansiona lazy la cartella ricordata al primo accesso, così la UI si popola in un colpo solo), `SelectFolder`/`SetFolder`, `Scan`, `ProcessAll` (normalizza + scrive tag), `ChooseDirectory` (dialog destinazione), `SetOptions`, `SetConfig`/`ResetConfig`/`SetAsDefault`, `SetWatchEnabled`, `ClearLogs`.

**`internal/`**:
- **`rules`** — `Config` (regole configurabili: estensioni supportate, occorrenze da rimuovere, alias di "ft", sostituzioni From→To) e `NormalizeFileBase()`, la **pipeline di normalizzazione** del nome (stesso ordine del Java originale: rimozione occorrenze → alias ft → `(ft` → sostituzioni → rimozione `[...]` → collapse spazi/trim → dash iniziale). `FactoryConfig()` è il seed di fabbrica.
- **`parser`** — estrazione di estensione, base name, e **titolo/artista per i tag** dal nome file (gestisce ` - `, ` ft `, remix, VIP, ecc.).
- **`fs`** — `ScanAudioFiles` (elenca i file audio supportati nella cartella, **non ricorsivo**), `IsDir`.
- **`rename`** — `Service.Process()`: calcola i nomi di destinazione, risolve le collisioni nel batch (un vincitore per nome), sposta/copia (`DeleteOriginals` on/off) e scrive i tag MP3. `Options { DestinationFolder, DeleteOriginals }`.
- **`tags`** — scrittura tag ID3 MP3.
- **`settings`** — persistenza JSON in `%AppData%\RenameMusic\`: `config.json` (regole correnti), `defaults.json` (default editabili), `state.json` (cartella, destinazione, elimina-originali, aggiornamento automatico). `Config` con campi mancanti eredita i valori di fabbrica.
- **`watcher`** — wrapper `fsnotify` per l'**aggiornamento automatico** della cartella sorgente.

**`frontend/src/App.tsx`** — unico componente principale. `guard()` avvolge le azioni async (imposta `busy`, gestisce errori, garantisce una **durata minima** del busy per non far lampeggiare la barra). Eventi Wails: `watch:changed` aggiorna solo l'anteprima.

## Concetti chiave / invarianti

- **Regole correnti vs default**: le "correnti" (`config.json`) sono attive; i "default" (`defaults.json`) sono un preset ripristinabile. "Salva come predefinito" sovrascrive i default; "Ripristina default" copia i default nelle correnti. La cartella si gestisce a parte (`state.json`), mai dentro le regole.
- **Aggiornamento automatico** (ex "watch"): osserva la cartella e aggiorna l'**anteprima** quando cambia il contenuto; **non converte mai** automaticamente. Dopo un `ProcessAll` il watcher va in pausa (`watchPaused`) fino al prossimo Scan, per ignorare gli eventi fsnotify auto-generati. In tutte le label UI il termine è **"Aggiornamento automatico"** (o "Agg. automatico"), non "watch".
- **Formato file**: i file trattati sono **sempre e solo mp3** (nessuna conversione fra formati diversi). La UI mostra per riga un chip blu con l'estensione (`ExtChip`), perché **l'estensione non è mai mostrata nei nomi file**, solo nel chip. `rename` mantiene comunque l'estensione del file (sorgente = destinazione).
- **Log strutturati**: le righe di attività sono `LogEntry { time, kind, message }` con `LogKind` = `info | success | error | auto`, assegnato **alla sorgente** in `addLogLocked(kind, message)`. Il frontend le rende direttamente (niente parsing/euristiche sul testo). Max 12 righe, più recenti in cima.

## Convenzioni

- Commenti e stringhe UI/log **in italiano**.
- Riferimenti a file clickabili come `path:line`.
- `docs/` è locale e ignorata da git (`.gitignore`).
- Non committare/mettere in stage senza richiesta esplicita.
