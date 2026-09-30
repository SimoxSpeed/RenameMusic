package com.renamemusic.app;

import android.content.Context;
import android.media.MediaScannerConnection;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.File;
import java.util.LinkedHashSet;
import java.util.Set;

/**
 * Aggiorna la libreria multimediale di Android (MediaStore) dopo che il core ha
 * rinominato, taggato o scaricato dei file: i lettori musicali leggono titolo e
 * artista da lì, non dai file, e senza una nuova scansione mostrerebbero ancora
 * i vecchi nomi/tag. Scansionare un percorso che non esiste più (il nome
 * originale di un file rinominato) lo rimuove dalla libreria.
 */
final class MediaRescan {

    private MediaRescan() {}

    /** Metodi del core dopo i quali i file su disco possono essere cambiati. */
    static boolean concerns(String method) {
        switch (method) {
            case "ProcessAll":
            case "ResolveTagPrompt":
            case "ClearTags":
            case "DownloadPlaylist":
            case "DownloadAndProcess":
            case "DownloadLink":
            case "DownloadLinkAndProcess":
                return true;
            default:
                return false;
        }
    }

    /** Ricava dalla risposta JSON del core (ActionResponse) i file coinvolti e li riscansiona. */
    static void afterCall(Context context, String method, String resultJSON) {
        try {
            JSONObject resp = new JSONObject(resultJSON);
            JSONObject state = resp.optJSONObject("state");
            if (state == null) return;
            String folder = state.optString("folder", "");
            String dest = state.optBoolean("destinationSameAsSource", true)
                ? folder
                : state.optString("destinationFolder", folder);

            Set<String> paths = new LinkedHashSet<>();
            JSONArray results = resp.optJSONArray("results");
            if (results != null) {
                // Conversione: vecchio nome (sparisce se spostato) e nuovo nome.
                for (int i = 0; i < results.length(); i++) {
                    JSONObject r = results.optJSONObject(i);
                    if (r == null) continue;
                    String oldName = r.optString("oldName", "");
                    String newName = r.optString("newName", "");
                    if (!oldName.isEmpty() && !folder.isEmpty()) paths.add(new File(folder, oldName).getPath());
                    if (!newName.isEmpty() && !dest.isEmpty()) paths.add(new File(dest, newName).getPath());
                }
            }
            if ("ClearTags".equals(method) || method.startsWith("Download")) {
                // Tag cancellati / nuovi file scaricati: tutti i file della cartella
                // (dopo una conversione l'elenco è vuoto e bastano i risultati).
                JSONArray files = state.optJSONArray("files");
                if (files != null) {
                    for (int i = 0; i < files.length(); i++) {
                        JSONObject f = files.optJSONObject(i);
                        if (f != null && !f.optString("path", "").isEmpty()) paths.add(f.optString("path"));
                    }
                }
            }
            if (!paths.isEmpty()) {
                MediaScannerConnection.scanFile(context, paths.toArray(new String[0]), null, null);
            }
        } catch (Exception ignored) {
            // La riscansione è un di più: un errore qui non deve toccare l'esito dell'operazione.
        }
    }
}
