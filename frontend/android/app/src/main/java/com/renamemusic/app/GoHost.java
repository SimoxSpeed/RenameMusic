package com.renamemusic.app;

import android.content.Context;
import android.util.Log;

import com.renamemusic.gobind.mobile.Host;
import com.renamemusic.gobind.mobile.Mobile;
import com.yausername.ffmpeg.FFmpeg;
import com.yausername.youtubedl_android.YoutubeDL;
import com.yausername.youtubedl_android.YoutubeDLRequest;
import com.yausername.youtubedl_android.YoutubeDLResponse;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

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
