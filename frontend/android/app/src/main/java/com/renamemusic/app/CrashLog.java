package com.renamemusic.app;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;

import androidx.core.content.FileProvider;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.FileWriter;
import java.io.IOException;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.nio.charset.StandardCharsets;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;

/**
 * Registro dei crash dell'app: eccezioni Java non gestite, errori fatali del
 * core Go (Mobile.setCrashOutput scrive nello stesso file) e crash della
 * WebView. Al successivo avvio la UI propone di condividerlo (es. su WhatsApp)
 * come file di testo.
 */
final class CrashLog {

    /** Oltre questa dimensione il registro riparte da capo. */
    private static final long MAX_BYTES = 512 * 1024;

    /** Sottocartella della cache esposta dal FileProvider (res/xml/file_paths.xml). */
    private static final String SHARE_DIR = "share";

    private CrashLog() {}

    static File file(Context context) {
        return new File(context.getFilesDir(), "crash.txt");
    }

    static boolean exists(Context context) {
        return file(context).length() > 0;
    }

    /** Aggiunge al registro un crash con l'eventuale eccezione (mai lancia). */
    static synchronized void write(Context context, String what, Throwable error) {
        try {
            File f = file(context);
            try (PrintWriter out = new PrintWriter(new FileWriter(f, f.length() < MAX_BYTES))) {
                out.println("=== " + new SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.ROOT).format(new Date()));
                out.println(what);
                if (error != null) {
                    StringWriter sw = new StringWriter();
                    error.printStackTrace(new PrintWriter(sw));
                    out.println(sw);
                }
                out.println();
            }
        } catch (Throwable ignored) {
            // il registro non deve mai causare a sua volta un crash
        }
    }

    /**
     * Condivide il registro (menu di condivisione di Android) e lo elimina: la
     * copia condivisa sta nella cache, con intestazione su app e telefono.
     */
    static void share(Activity activity, String appVersion) throws IOException {
        File dir = new File(activity.getCacheDir(), SHARE_DIR);
        if (!dir.isDirectory() && !dir.mkdirs()) throw new IOException("cartella non creata");
        String stamp = new SimpleDateFormat("yyyyMMdd-HHmm", Locale.ROOT).format(new Date());
        File out = new File(dir, "RenameMusic-errore-" + stamp + ".txt");
        try (FileOutputStream os = new FileOutputStream(out);
             FileInputStream in = new FileInputStream(file(activity))) {
            String header = "RenameMusic " + appVersion + "\n"
                + Build.MANUFACTURER + " " + Build.MODEL
                + " · Android " + Build.VERSION.RELEASE + " (API " + Build.VERSION.SDK_INT + ")"
                + " · " + Build.SUPPORTED_ABIS[0] + "\n\n";
            os.write(header.getBytes(StandardCharsets.UTF_8));
            byte[] buf = new byte[8192];
            for (int n; (n = in.read(buf)) > 0; ) os.write(buf, 0, n);
        }
        discard(activity);

        Uri uri = FileProvider.getUriForFile(activity, activity.getPackageName() + ".fileprovider", out);
        Intent send = new Intent(Intent.ACTION_SEND)
            .setType("text/plain")
            .putExtra(Intent.EXTRA_STREAM, uri)
            .putExtra(Intent.EXTRA_SUBJECT, "Errore di RenameMusic")
            .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        activity.startActivity(Intent.createChooser(send, "Condividi errore"));
    }

    /**
     * Svuota il registro senza eliminarlo: il runtime Go ci tiene aperto un
     * descrittore (Mobile.setCrashOutput) e un file eliminato gli farebbe
     * perdere i crash successivi della stessa sessione.
     */
    static synchronized void discard(Context context) {
        try {
            new FileOutputStream(file(context)).close();
        } catch (IOException ignored) {
            // registro già assente
        }
    }

    /**
     * Registra le eccezioni non gestite di qualunque thread Java, poi lascia
     * proseguire il gestore di sistema (che chiude l'app come prima).
     */
    static void install(Context context) {
        Thread.UncaughtExceptionHandler previous = Thread.getDefaultUncaughtExceptionHandler();
        if (previous instanceof Handler) return;
        Thread.setDefaultUncaughtExceptionHandler(new Handler(context.getApplicationContext(), previous));
    }

    private static final class Handler implements Thread.UncaughtExceptionHandler {
        private final Context context;
        private final Thread.UncaughtExceptionHandler previous;

        Handler(Context context, Thread.UncaughtExceptionHandler previous) {
            this.context = context;
            this.previous = previous;
        }

        @Override
        public void uncaughtException(Thread thread, Throwable error) {
            write(context, "Eccezione non gestita nel thread " + thread.getName(), error);
            if (previous != null) previous.uncaughtException(thread, error);
        }
    }
}
