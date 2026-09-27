package com.renamemusic.app;

import android.Manifest;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.Settings;
import android.util.Log;

import com.getcapacitor.JSObject;
import com.getcapacitor.PermissionState;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.getcapacitor.annotation.Permission;
import com.getcapacitor.annotation.PermissionCallback;
import com.renamemusic.gobind.mobile.Mobile;

import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * Ponte tra la UI (WebView) e il core Go (libreria gomobile). È l'equivalente
 * Android dei binding Wails: la UI chiama {@code call(method, args)} e riceve il
 * risultato JSON; gli eventi del core arrivano come evento "event".
 *
 * Gestisce anche ciò che è solo Android: il permesso di accesso a tutti i file e
 * il servizio in primo piano che tiene vivo il processo durante le operazioni
 * lunghe (conversione, download di playlist).
 */
@CapacitorPlugin(
    name = "RenameMusic",
    permissions = {
        @Permission(
            alias = "legacyStorage",
            strings = { Manifest.permission.READ_EXTERNAL_STORAGE, Manifest.permission.WRITE_EXTERNAL_STORAGE }
        ),
        @Permission(alias = "notifications", strings = { Manifest.permission.POST_NOTIFICATIONS }),
    }
)
public class RenameMusicPlugin extends Plugin {

    private static final String TAG = "RenameMusic";

    /**
     * Metodi del core che possono durare a lungo: finché uno è in corso teniamo
     * attivo il servizio in primo piano, così Android non chiude l'app se
     * l'utente la mette in background (es. durante il download di una playlist).
     */
    private static final Set<String> LONG_METHODS = new HashSet<>(
        Arrays.asList("ProcessAll", "ClearTags", "DownloadPlaylist", "InstallYtDlp")
    );

    /**
     * Le chiamate del plugin arrivano tutte su un unico thread di Capacitor:
     * eseguendole lì, una conversione lunga bloccherebbe anche "Annulla". Le
     * smistiamo quindi su un pool, come fa Wails su desktop (il core è
     * thread-safe).
     */
    private final ExecutorService executor = Executors.newCachedThreadPool();

    private final AtomicInteger longOps = new AtomicInteger();

    private GoHost host;

    @Override
    public void load() {
        host = new GoHost(getContext().getApplicationContext(), this::onCoreEvent);
        try {
            Mobile.start(getContext().getFilesDir().getAbsolutePath(), host);
        } catch (Exception e) {
            Log.e(TAG, "avvio del core fallito", e);
        }
        // youtubedl-android estrae Python al primo avvio (qualche secondo): lo
        // inizializziamo in background e il core notifica la UI a fine init.
        host.initYtDlpAsync();
    }

    /** Evento del core: lo inoltriamo alla UI e aggiorniamo la notifica di avanzamento. */
    private void onCoreEvent(String name, String payloadJSON) {
        JSObject data = new JSObject();
        data.put("name", name);
        data.put("payload", payloadJSON);
        notifyListeners("event", data);
        if ("process:progress".equals(name) && longOps.get() > 0) {
            WorkService.updateProgress(getContext(), payloadJSON);
        }
    }

    @PluginMethod
    public void call(PluginCall call) {
        String method = call.getString("method", "");
        String args = call.getString("args", "[]");
        boolean isLong = LONG_METHODS.contains(method);
        if (isLong && longOps.getAndIncrement() == 0) {
            WorkService.start(getContext(), titleFor(method));
        }
        executor.execute(() -> {
            try {
                String result = Mobile.call(method, args);
                JSObject ret = new JSObject();
                ret.put("result", result);
                call.resolve(ret);
                if (MediaRescan.concerns(method)) {
                    MediaRescan.afterCall(getContext(), method, result);
                }
            } catch (Exception e) {
                call.reject(e.getMessage() != null ? e.getMessage() : e.toString());
            } finally {
                if (isLong && longOps.decrementAndGet() == 0) {
                    WorkService.stop(getContext());
                }
            }
        });
    }

    private static String titleFor(String method) {
        switch (method) {
            case "DownloadPlaylist":
                return "Download della playlist in corso";
            case "ClearTags":
                return "Cancellazione dei tag in corso";
            case "InstallYtDlp":
                return "Aggiornamento di yt-dlp in corso";
            default:
                return "Conversione in corso";
        }
    }

    // ---- Accesso ai file ----------------------------------------------------

    /**
     * L'app lavora su percorsi reali (come su desktop): serve l'accesso a tutti
     * i file (Android 11+) o i permessi di archiviazione classici (Android 10-).
     */
    private boolean hasStorageAccess() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            return Environment.isExternalStorageManager();
        }
        return getPermissionState("legacyStorage") == PermissionState.GRANTED;
    }

    @PluginMethod
    public void storageStatus(PluginCall call) {
        JSObject ret = new JSObject();
        ret.put("granted", hasStorageAccess());
        call.resolve(ret);
    }

    /**
     * Su Android 11+ apre la schermata di sistema "Accesso a tutti i file" (la
     * UI ricontrolla lo stato al ritorno nell'app, vedi handleOnResume); su
     * Android 10- mostra la classica richiesta di permesso.
     */
    @PluginMethod
    public void requestStorage(PluginCall call) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            try {
                Intent intent = new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION);
                intent.setData(Uri.parse("package:" + getContext().getPackageName()));
                getActivity().startActivity(intent);
            } catch (Exception e) {
                getActivity().startActivity(new Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION));
            }
            JSObject ret = new JSObject();
            ret.put("granted", hasStorageAccess());
            call.resolve(ret);
            return;
        }
        requestPermissionForAlias("legacyStorage", call, "legacyStorageCallback");
    }

    @PermissionCallback
    private void legacyStorageCallback(PluginCall call) {
        JSObject ret = new JSObject();
        ret.put("granted", hasStorageAccess());
        call.resolve(ret);
        emitResume();
    }

    /**
     * Permesso per la notifica del servizio in primo piano (Android 13+). Se
     * negato il servizio funziona comunque, solo senza notifica visibile.
     */
    @PluginMethod
    public void requestNotifications(PluginCall call) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU
            || getPermissionState("notifications") == PermissionState.GRANTED) {
            call.resolve();
            return;
        }
        requestPermissionForAlias("notifications", call, "notificationsCallback");
    }

    @PermissionCallback
    private void notificationsCallback(PluginCall call) {
        call.resolve();
    }

    // ---- Ciclo di vita --------------------------------------------------------

    @Override
    protected void handleOnResume() {
        super.handleOnResume();
        // Al ritorno in primo piano: il core riscansiona se l'aggiornamento
        // automatico è attivo, e la UI ricontrolla il permesso sui file (l'utente
        // potrebbe averlo appena concesso dalle impostazioni di sistema).
        executor.execute(Mobile::resume);
        emitResume();
    }

    private void emitResume() {
        JSObject data = new JSObject();
        data.put("storageGranted", hasStorageAccess());
        notifyListeners("resume", data);
    }
}
