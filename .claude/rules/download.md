---
paths:
  - "internal/playlist/**"
  - "internal/core/ytdlp_update.go"
  - "mobile/ytdlp.go"
  - "frontend/android/app/src/main/java/com/renamemusic/app/GoHost.java"
  - "frontend/src/main/DownloadCommands.tsx"
  - "frontend/src/settings/DownloadTab.tsx"
---

# Download con yt-dlp e ffmpeg

Download di playlist e link con yt-dlp (copia gestita, aggiornamento automatico, file temporanei) e ffmpeg.

- **`playlist`** — download di playlist con yt-dlp. `Runner` astrae l'esecuzione (`ExecRunner` su desktop, bridge verso youtubedl-android su Android): enumerazione, concorrenza ed estrazione errori sono comuni. yt-dlp scrive i file intermedi (`.part`, audio prima della conversione) in `<cartella>/.renamemusic-download/<id video>` (`-P temp:`; il modello `-o` deve restare relativo, con `-P home:`) e sposta nella cartella solo l'mp3 finito: la sottocartella si elimina a fine download di ogni video, a inizio e fine playlist (`CleanTempDir`) e all'avvio (`core.Start`). Su desktop anche ffmpeg (necessario per l'mp3) ha una copia gestita in `%AppData%\RenameMusic\ffmpeg` (build di `yt-dlp/FFmpeg-Builds`, scaricata da `InstallFFmpeg`, verificata come yt-dlp con lo sha256 pubblicato nella release (`checksums.sha256`/`SHA2-256SUMS`, `internal/playlist/checksum.go`) e rimovibile a parte con `UninstallFFmpeg`, che `UninstallYtDlp` non tocca; la UI la scarica anche prima del download di una playlist, se manca), passata a yt-dlp con `--ffmpeg-location`; senza di essa si usa l'ffmpeg del PATH. Con yt-dlp gestito il core lo **aggiorna da sé** (`internal/core/ytdlp_update.go`: all'avvio e ogni 24 ore, `YtDlp.Update`), mai durante un download (`ytDlpMu`): una versione vecchia di qualche settimana dà `HTTP Error 403`.
- **Link singoli**: il core accetta solo link `http(s)` (un testo che inizia con `-` sarebbe un'opzione di yt-dlp) e passa `--no-playlist` all'enumerazione (`playlist.Options.Single`), così un video aperto dentro una playlist (`watch?v=...&list=...`) scarica solo il video.
