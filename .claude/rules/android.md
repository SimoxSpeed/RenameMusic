---
paths:
  - "mobile/**"
  - "frontend/android/**"
  - "frontend/capacitor.config.ts"
  - "frontend/src/api.ts"
  - "frontend/src/mobile.css"
  - "frontend/src/FolderPicker.tsx"
---

# Android

Dettagli dell'app Android: package `mobile` (gomobile), codice Java del progetto Capacitor, differenze di UI.

**Android** — `mobile/` è il package per `gomobile bind`: `Start(dataDir, host)`, `Call(method, argsJSON)` (dispatch via reflection sui metodi di `core.App` + quelli di `folderAPI`, argomenti/risultati JSON: l'equivalente dei binding Wails), `Resume()`, `YtDlpInitialized()`. `Host` è implementata in Java: `frontend/android/app/src/main/java/com/renamemusic/app/` — `RenameMusicPlugin` (plugin Capacitor: smista le chiamate su un pool di thread, permesso "accesso a tutti i file", servizio in primo piano durante le operazioni lunghe, riscansione MediaStore, lettura degli appunti per il tasto «Incolla»), `GoHost` (eventi + yt-dlp via youtubedl-android con gli stessi argomenti del desktop + installer degli aggiornamenti, `InstallApk`), `WorkService`, `MediaRescan`. `Updater` Android in `mobile/update.go`. I crash (eccezioni Java non gestite, errori fatali di Go via `Mobile.setCrashOutput`, morte del processo della WebView, che `MainActivity` gestisce ricreando l'Activity) finiscono in `files/crash.txt` (`CrashLog`): al successivo avvio la UI mostra il popup "L'app si è chiusa in modo anomalo" con **Condividi errore** (menu di condivisione di Android con un .txt, copiato in `cache/share/` ed esposto dal FileProvider); poi il registro si svuota, senza eliminarlo perché il runtime Go ci tiene aperto un descrittore. I metodi di `GoHost` chiamati da Go non devono mai lanciare (nemmeno `Error`: `catch (Throwable)`), perché gomobile lascia l'eccezione pendente e il processo abortisce; le goroutine di background del core passano da `safely` (`internal/core/recover.go`).

**`frontend/src/api.ts`** — unico accesso della UI al core: su desktop `window.go.core.App`, su Android il plugin `RenameMusic` (`isAndroid`). Le differenze di UI Android sono condizionate a `isAndroid` (desktop invariato): selettore cartelle interno (`frontend/src/FolderPicker.tsx`), banner per il permesso sui file, tasto Indietro, pull to refresh, tasto «Incolla» nel campo del link, yt-dlp integrato (solo "Aggiorna"), layout mobile in `frontend/src/mobile.css` (tutto sotto `.is-android`).

- **Android**: l'app è installata a mano (mai sul Play Store), quindi usa `MANAGE_EXTERNAL_STORAGE` e lavora su **percorsi reali** come il desktop (niente SAF/URI `content://`). Il core Go è compilato solo per `arm64-v8a` e `x86_64`: gli APK sono divisi per ABI (`splits` in `app/build.gradle`).
