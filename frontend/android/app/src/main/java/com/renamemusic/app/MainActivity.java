package com.renamemusic.app;

import android.os.Build;
import android.os.Bundle;
import android.os.SystemClock;
import android.webkit.RenderProcessGoneDetail;
import android.webkit.WebView;

import com.getcapacitor.BridgeActivity;
import com.getcapacitor.PluginHandle;
import com.getcapacitor.WebViewListener;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        CrashLog.install(this);
        // Il plugin locale (ponte verso il core Go) va registrato prima che il
        // bridge venga creato in super.onCreate.
        registerPlugin(RenameMusicPlugin.class);
        super.onCreate(savedInstanceState);
        bridge.addWebViewListener(new RenderRecovery(this));
    }

    /** Con il focus si possono leggere gli appunti: vedi RenameMusicPlugin.onWindowFocused. */
    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (!hasFocus || bridge == null) return;
        PluginHandle handle = bridge.getPlugin("RenameMusic");
        if (handle != null && handle.getInstance() instanceof RenameMusicPlugin) {
            ((RenameMusicPlugin) handle.getInstance()).onWindowFocused();
        }
    }

    /**
     * Il motore della WebView gira in un processo separato: se termina (crash
     * del driver grafico su alcuni telefoni, memoria esaurita) e nessuno lo
     * gestisce, Android chiude l'intera app. Lo registriamo e ricreiamo
     * l'Activity con una WebView nuova: il core Go resta vivo nel processo,
     * quindi la UI si ripopola dallo stato corrente.
     */
    private static final class RenderRecovery extends WebViewListener {
        /**
         * Se la WebView termina di nuovo subito dopo essere stata ricreata non
         * ci riproviamo (sarebbe un ciclo infinito): Android chiude l'app.
         */
        private static final long MIN_INTERVAL_MS = 10_000;
        private static long lastRecovery;

        private final MainActivity activity;

        RenderRecovery(MainActivity activity) {
            this.activity = activity;
        }

        @Override
        public boolean onRenderProcessGone(WebView webView, RenderProcessGoneDetail detail) {
            String why = "Processo della WebView terminato";
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && detail != null) {
                why += detail.didCrash() ? " (crash)" : " (chiuso dal sistema per liberare memoria)";
            }
            CrashLog.write(activity, why, null);
            long now = SystemClock.elapsedRealtime();
            if (lastRecovery != 0 && now - lastRecovery < MIN_INTERVAL_MS) return false;
            lastRecovery = now;
            activity.recreate();
            return true;
        }
    }
}
