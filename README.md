# RenameMusic

App per **Windows** e **Android** che normalizza i nomi dei file musicali e ne scrive i tag ID3 (titolo e artista dedotti dal nome), con il download delle playlist di YouTube tramite yt-dlp.

- Anteprima dei nuovi nomi prima di toccare qualunque file: nulla cambia senza un comando esplicito.
- Regole di normalizzazione configurabili (occorrenze da rimuovere, alias di "ft", sostituzioni), con predefiniti ripristinabili.
- Download di playlist salvate o di un singolo link, con conversione in mp3 e proposte dei nomi da MusicBrainz per le tracce dubbie.
- Account Google facoltativo: playlist di YouTube dell'account e impostazioni condivise fra i dispositivi. Vedi l'[informativa sulla privacy](PRIVACY.md).

## Installazione

Scarica dall'[ultima release](https://github.com/SimoxSpeed/RenameMusic/releases/latest):

- Windows: `RenameMusic.exe`
- Android: `RenameMusic-arm64-v8a.apk` (telefoni) o `RenameMusic-x86_64.apk` (emulatore), da installare a mano

L'app controlla da sola se c'è una nuova versione e si aggiorna dalle release.

## Compilare dai sorgenti

Il core è in Go ed è condiviso. La GUI è la stessa sulle due piattaforme: React + TypeScript, impacchettata con [Wails v2](https://wails.io) su desktop e con [Capacitor](https://capacitorjs.com) su Android.

### Requisiti

- Go 1.25+
- Node.js e npm
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`
- Solo per Android:
  - Android SDK con NDK
  - JDK 21
  - `gomobile` e `gobind` alla stessa versione di `golang.org/x/mobile` in [go.mod](go.mod)

### Desktop

Tutti i comandi si lanciano dalla radice del repo:

```powershell
npm run setup  # dipendenze del frontend
npm run dev    # sviluppo con hot reload (wails dev)
npm run build  # build\bin\RenameMusic.exe (wails build)
npm test       # go test ./... (compila prima il frontend se manca)
npm run check  # tsc + build del frontend, go vet e test
```

Senza credenziali OAuth la parte "Account Google" non è disponibile (il resto dell'app funziona). Per abilitarla copia [credentials_local.go.example](internal/google/credentials_local.go.example) in `internal/google/credentials_local.go`, ignorato da git, e inserisci il client OAuth «App desktop» di Google Cloud. In alternativa puoi impostare le variabili `RENAMEMUSIC_GOOGLE_CLIENT_ID` / `RENAMEMUSIC_GOOGLE_CLIENT_SECRET`.

### Android

Copia [keystore.properties.example](frontend/android/keystore.properties.example) in `frontend/android/keystore.properties`, ignorato da git, e indica il keystore con cui firmare gli APK. Poi:

```powershell
npm run apk:build    # build\bin\RenameMusic-<abi>.apk
npm run apk:install  # installa sul dispositivo collegato con adb
npm run apk          # tutte e due
```

Gli aggiornamenti in-app funzionano solo fra APK con la stessa firma. Per questo la build accetta solo il certificato delle versioni pubblicate (`$SigningCertSha1` in [mobile/signing.ps1](mobile/signing.ps1)). Con un tuo keystore cambia quel valore: l'app risultante va installata da zero e non riceve gli aggiornamenti delle release ufficiali.

### Release

`npm run release -- patch|minor|major` incrementa la versione, compila exe e APK, crea commit, tag e release su GitHub. I dettagli sono in testa a [release.ps1](release.ps1).
