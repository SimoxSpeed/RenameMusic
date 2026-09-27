package com.renamemusic.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;

import androidx.core.app.NotificationCompat;
import androidx.core.content.ContextCompat;

import org.json.JSONObject;

/**
 * Servizio in primo piano attivo solo durante le operazioni lunghe del core
 * (conversione, cancellazione tag, download di playlist). Non esegue lavoro:
 * il lavoro gira nel core Go. Serve a far sapere ad Android che l'app sta
 * lavorando, così non ne chiude il processo se l'utente passa ad altro, e tiene
 * sveglia la CPU (wake lock) anche a schermo spento. La notifica mostra
 * l'avanzamento "x / totale" ricevuto dagli eventi process:progress.
 */
public class WorkService extends Service {

    private static final String CHANNEL_ID = "work";
    private static final int NOTIFICATION_ID = 1;
    private static final String EXTRA_TITLE = "title";

    /** Limite di sicurezza del wake lock: un download di playlist lunghissimo resta coperto. */
    private static final long WAKE_LOCK_TIMEOUT_MS = 6L * 60 * 60 * 1000;

    /** true tra onCreate e onDestroy: l'avanzamento aggiorna la notifica solo a servizio attivo. */
    private static volatile boolean running = false;
    private static volatile String title = "Operazione in corso";

    private PowerManager.WakeLock wakeLock;

    static void start(Context context, String operationTitle) {
        Intent intent = new Intent(context, WorkService.class).putExtra(EXTRA_TITLE, operationTitle);
        try {
            ContextCompat.startForegroundService(context, intent);
        } catch (Exception ignored) {
            // Avvio non consentito (es. app già in background): l'operazione
            // prosegue comunque, solo senza protezione del servizio.
        }
    }

    static void stop(Context context) {
        context.stopService(new Intent(context, WorkService.class));
    }

    /** Aggiorna la notifica del servizio con l'avanzamento (payload di process:progress). */
    static void updateProgress(Context context, String payloadJSON) {
        if (!running) return;
        try {
            JSONObject p = new JSONObject(payloadJSON);
            NotificationManager nm = (NotificationManager) context.getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm != null && running) {
                nm.notify(NOTIFICATION_ID, build(context, p.optInt("done"), p.optInt("total")));
            }
        } catch (Exception ignored) {
            // avanzamento non essenziale
        }
    }

    @Override
    public void onCreate() {
        super.onCreate();
        running = true;
        createChannel(this);
        PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
        if (pm != null) {
            wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "RenameMusic:work");
            wakeLock.setReferenceCounted(false);
            wakeLock.acquire(WAKE_LOCK_TIMEOUT_MS);
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null && intent.getStringExtra(EXTRA_TITLE) != null) {
            title = intent.getStringExtra(EXTRA_TITLE);
        }
        Notification notification = build(this, 0, 0);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
        } else {
            startForeground(NOTIFICATION_ID, notification);
        }
        return START_NOT_STICKY;
    }

    private static Notification build(Context context, int done, int total) {
        Intent open = new Intent(context, MainActivity.class)
            .setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        PendingIntent pi = PendingIntent.getActivity(context, 0, open, PendingIntent.FLAG_IMMUTABLE);
        NotificationCompat.Builder b = new NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_sys_download)
            .setContentTitle(title)
            .setContentIntent(pi)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true);
        if (total > 0) {
            b.setContentText(done + " / " + total + " completati").setProgress(total, done, false);
        } else {
            b.setContentText("Operazione in corso…").setProgress(0, 0, true);
        }
        return b.build();
    }

    private static void createChannel(Context context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return;
        NotificationManager nm = (NotificationManager) context.getSystemService(Context.NOTIFICATION_SERVICE);
        if (nm == null) return;
        NotificationChannel ch = new NotificationChannel(
            CHANNEL_ID, "Operazioni in corso", NotificationManager.IMPORTANCE_LOW);
        ch.setDescription("Avanzamento di conversioni e download");
        nm.createNotificationChannel(ch);
    }

    @Override
    public void onDestroy() {
        running = false;
        if (wakeLock != null && wakeLock.isHeld()) {
            wakeLock.release();
        }
        stopForeground(STOP_FOREGROUND_REMOVE);
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
