package com.renamemusic.app;

import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.provider.Settings;
import android.util.Log;

import androidx.core.content.FileProvider;

import com.renamemusic.gobind.mobile.Host;
import com.renamemusic.gobind.mobile.Mobile;
import com.yausername.ffmpeg.FFmpeg;
import com.yausername.youtubedl_android.YoutubeDL;
import com.yausername.youtubedl_android.YoutubeDLRequest;
import com.yausername.youtubedl_android.YoutubeDLResponse;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.io.File;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;

/**
 * Implementazione Java dell'interfaccia Host del core Go (package mobile):
 * inoltra gli eventi del core alla UI ed esegue yt-dlp tramite
 * youtubedl-android (Python + ffmpeg incorporati nell'APK). Su desktop lo stesso
 * ruolo lo svolge l'eseguibile yt-dlp.exe: gli argomenti passati dal core sono
 * identici.
 */
final class GoHost implements Host {

    private static final String TAG = "RenameMusic";

    /** Destinatario degli eventi del core (il plugin, che li gira alla WebView). */
    interface EventSink {
        void emit(String name, String payloadJSON);
    }

    private final Context context;
    private final EventSink sink;

    /** Si apre al termine (riuscito o meno) dell'inizializzazione di youtubedl-android. */
    private final CountDownLatch initDone = new CountDownLatch(1);
    private volatile boolean ready = false;

    /**
     * APK in attesa del permesso "installa app sconosciute": l'installazione
     * riparte al ritorno nell'app (vedi {@link #resumePendingInstall()}).
     */
    private volatile String pendingApk;

    GoHost(Context context, EventSink sink) {
        this.context = context;
        this.sink = sink;
    }

    /** Avvia in background l'inizializzazione di youtubedl-android e ffmpeg. */
    void initYtDlpAsync() {
        new Thread(() -> {
            try {
                initYtDlp();
            } finally {
                initDone.countDown();
                Mobile.ytDlpInitialized();
            }
        }, "ytdlp-init").start();
    }

    private synchronized boolean initYtDlp() {
        if (ready) return true;
        try {
            YoutubeDL.getInstance().init(context);
            FFmpeg.getInstance().init(context);
            ready = true;
        } catch (Exception e) {
            Log.e(TAG, "inizializzazione di youtubedl-android fallita", e);
        }
        return ready;
    }

    private void awaitInit() {
        try {
            initDone.await();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    @Override
    public void emit(String event, String payloadJSON) {
        sink.emit(event, payloadJSON);
    }

    @Override
    public boolean ytDlpReady() {
        return ready;
    }

    @Override
    public String ytDlpRun(String argsJSON) {
        awaitInit();
        JSONObject out = new JSONObject();
        try {
            if (!ready && !initYtDlp()) {
                return result(out, "", "", "yt-dlp non inizializzato");
            }
            JSONArray array = new JSONArray(argsJSON);
            List<String> args = new ArrayList<>(array.length());
            for (int i = 0; i < array.length(); i++) {
                args.add(array.getString(i));
            }
            // Nessun URL "separato": gli argomenti del core (URL compreso) sono
            // passati così come sono, dopo le opzioni che la libreria aggiunge da
            // sé (--ffmpeg-location, --js-runtimes, --no-cache-dir).
            YoutubeDLRequest request = new YoutubeDLRequest(new ArrayList<String>());
            request.addCommands(args);
            YoutubeDLResponse response = YoutubeDL.getInstance().execute(request, null, null);
            return result(out, response.getOut(), response.getErr(), "");
        } catch (Exception e) {
            // Con exit code != 0 la libreria lancia un'eccezione il cui messaggio
            // è lo stderr di yt-dlp: il core ne estrae la riga "ERROR:".
            String msg = e.getMessage() != null ? e.getMessage() : e.toString();
            return result(out, "", msg, "yt-dlp terminato con errore");
        }
    }

    @Override
    public String ytDlpUpdate() {
        awaitInit();
        if (!ready && !initYtDlp()) {
            return "impossibile inizializzare yt-dlp";
        }
        try {
            YoutubeDL.getInstance().updateYoutubeDL(context, YoutubeDL.UpdateChannel._STABLE);
            return "";
        } catch (Exception e) {
            return e.getMessage() != null ? e.getMessage() : e.toString();
        }
    }

    /**
     * Aggiornamento dell'app: apre l'installer di sistema sull'APK scaricato dal
     * core. Se Android richiede prima il permesso di installare app da
     * RenameMusic apre la relativa schermata delle impostazioni e ricorda l'APK,
     * che viene installato al ritorno nell'app.
     */
    @Override
    public String installApk(String path) {
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
                && !context.getPackageManager().canRequestPackageInstalls()) {
                pendingApk = path;
                Intent intent = new Intent(
                    Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                    Uri.parse("package:" + context.getPackageName())
                );
                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                context.startActivity(intent);
                return Mobile.InstallPermission;
            }
            launchInstaller(path);
            return "";
        } catch (Exception e) {
            Log.e(TAG, "apertura dell'installer fallita", e);
            return e.getMessage() != null ? e.getMessage() : e.toString();
        }
    }

    /**
     * Al ritorno nell'app: se un aggiornamento aspettava il permesso e ora è
     * concesso, apre l'installer. Se il permesso è stato negato l'APK in attesa
     * viene dimenticato (l'utente può riprovare da "Aggiorna").
     */
    void resumePendingInstall() {
        String path = pendingApk;
        if (path == null) return;
        pendingApk = null;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
            && !context.getPackageManager().canRequestPackageInstalls()) {
            return;
        }
        try {
            launchInstaller(path);
        } catch (Exception e) {
            Log.e(TAG, "apertura dell'installer fallita", e);
        }
    }

    /** L'APK sta nella cartella privata dell'app: lo esponiamo con il FileProvider. */
    private void launchInstaller(String path) {
        Uri uri = FileProvider.getUriForFile(context, context.getPackageName() + ".fileprovider", new File(path));
        Intent intent = new Intent(Intent.ACTION_VIEW);
        intent.setDataAndType(uri, "application/vnd.android.package-archive");
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);
        context.startActivity(intent);
    }

    private static String result(JSONObject out, String stdout, String stderr, String error) {
        try {
            out.put("stdout", stdout);
            out.put("stderr", stderr);
            out.put("error", error);
        } catch (JSONException ignored) {
            // put con chiavi stringa non fallisce
        }
        return out.toString();
    }
}
