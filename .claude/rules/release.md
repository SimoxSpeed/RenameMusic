---
paths:
  - "release.ps1"
  - "mobile/*.ps1"
  - "internal/update/**"
  - "internal/core/update.go"
  - "update_desktop.go"
  - "mobile/update.go"
  - "frontend/android/app/build.gradle"
  - "frontend/android/keystore.properties*"
  - "frontend/src/dialogs/UpdateDialog.tsx"
  - "frontend/src/settings/InfoTab.tsx"
---

# Release, aggiornamenti dell'app e firma degli APK

Come si pubblica una versione, come l'app si aggiorna da sola e perché la firma degli APK non deve cambiare.

- **`npm run release -- patch|minor|major`** (`release.ps1`; l'argomento è **obbligatorio**, accettato anche come `-patch`/`--patch`): si ferma subito se manca `internal/google/credentials_local.go` o ha ancora i valori del modello (le variabili d'ambiente non contano) e prima del commit verifica che l'exe contenga il client ID; incrementa `update.Version`, compila exe + APK, e dopo conferma fa commit (messaggio fisso `chore(release): 🚀 release X.Y.Z`), tag `vX.Y.Z` e push (`--atomic`), poi crea la release su GitHub con `gh release create` (exe + APK allegati; le note si scrivono solo all'inizio dello script, in un file temporaneo vuoto aperto nel Blocco note che viene letto ed eliminato subito, e restano solo in memoria). Senza GitHub CLI autenticata apre invece la pagina della nuova release da completare a mano. Se il tag esiste già (in locale o su origin) chiede se ricrearlo o annullare: il vecchio tag si sostituisce solo al push (`git tag -f` + push forzato del solo tag), e con `gh` si elimina anche la release rimasta su quel tag; con `-Yes` invece si interrompe. Se la release si annulla o fallisce prima del commit, ripristina versione, staging ed exe/APK di `build\bin` (messi da parte in `build\bin\.release-backup`). `-NoBuild` salta le build, `-Yes` la conferma. Lo lancia **l'utente** (vedi "Commit" in CLAUDE.md).
- **`update`** — aggiornamenti dell'app: `Version` (**unica fonte della versione**, letta anche da `build-apk.ps1` per `versionName`/`versionCode`), `Latest` (ultima release GitHub di `SimoxSpeed/RenameMusic`), `Newer`, `Download` (verifica dimensione e sha256), `ReplaceExecutable`/`CleanupExecutable` (desktop: rename dell'exe in uso su `.old`). `RENAMEMUSIC_UPDATE_URL` sostituisce l'URL dell'API per provare con un server locale.
- **Aggiornamenti dell'app**: il core (se ha un `Updater`) controlla l'ultima release GitHub all'avvio, poi ogni 6 ore; se fallisce (offline) riprova ogni 2 minuti, e la UI ricontrolla subito all'evento `online`. Il popup "Nuova versione disponibile" compare **una sola volta per versione** (`MarkUpdateSeen` → `updateSeenVersion` in `state.json`); dopo resta il tasto "Aggiorna" nella scheda "Info" delle Impostazioni (e un pallino sul pulsante Impostazioni, che apre direttamente quella scheda, e sulla scheda). Desktop: scarica `RenameMusic.exe` accanto all'exe in uso, lo sostituisce e riavvia. Android: scarica `RenameMusic-<abi>.apk` in `files/update/` e apre l'installer (FileProvider + `REQUEST_INSTALL_PACKAGES`; se manca il permesso "installa app sconosciute" apre le impostazioni e installa al ritorno). L'APK nuovo deve avere la **stessa firma** (vedi "Firma degli APK" più sotto) e un `versionCode` maggiore.
- **Pubblicare una versione**: `release.ps1` (incrementa `update.Version`, `wails build` + `mobile\build-apk.ps1`, commit + tag `v<Version>` + push, release GitHub con `gh`). La release sul tag (non pre-release) deve allegare `build\bin\RenameMusic.exe`, `RenameMusic-arm64-v8a.apk` e `RenameMusic-x86_64.apk` con **esattamente questi nomi**: con `gh` lo fa lo script, senza va fatto a mano. Il testo della release (le note scritte all'inizio dello script) appare nel popup.
- **Firma degli APK**: il keystore (quello di debug con cui sono firmate tutte le versioni pubblicate) sta **fuori dal repo**; lo indica `frontend/android/keystore.properties` (ignorato da git, modello in `keystore.properties.example`; nei percorsi `/`, non `\`), letto da `app/build.gradle` per le build debug e release. Senza quel file Gradle si ferma a `preBuild` invece di firmare col `~/.android/debug.keystore` della macchina. Lo SHA-1 atteso del certificato è fissato in `mobile/signing.ps1` (`$SigningCertSha1`): `build-apk.ps1` controlla il keystore prima di compilare, `release.ps1` controlla `keystore.properties` al passo 1 e la firma degli APK da pubblicare (apksigner) prima del commit. La firma non deve cambiare mai: Android rifiuterebbe gli aggiornamenti e il client OAuth Android di Google (registrato con quello SHA-1) non riconoscerebbe l'app.
