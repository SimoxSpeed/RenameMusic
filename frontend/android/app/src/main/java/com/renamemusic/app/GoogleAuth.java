package com.renamemusic.app;

import android.app.Activity;
import android.app.PendingIntent;
import android.content.Context;
import android.util.Log;

import androidx.activity.result.ActivityResult;

import com.google.android.gms.auth.api.identity.AuthorizationClient;
import com.google.android.gms.auth.api.identity.AuthorizationRequest;
import com.google.android.gms.auth.api.identity.AuthorizationResult;
import com.google.android.gms.auth.api.identity.ClearTokenRequest;
import com.google.android.gms.auth.api.identity.Identity;
import com.google.android.gms.common.api.ApiException;
import com.google.android.gms.common.api.CommonStatusCodes;
import com.google.android.gms.common.api.Scope;
import com.google.android.gms.tasks.Tasks;

import org.json.JSONException;
import org.json.JSONObject;

import java.util.Arrays;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;

/**
 * Accesso all'account Google per la YouTube Data API tramite Google Play
 * Services (AuthorizationClient): è Play Services a conservare il permesso
 * concesso e a rinnovare l'access token. Il client OAuth Android del progetto
 * Google Cloud riconosce l'app dal nome del pacchetto e dall'impronta SHA-1
 * della firma, quindi nel codice non c'è alcuna credenziale.
 *
 * I metodi sono chiamati dal core Go (via GoHost) su thread in background e
 * possono bloccare: la schermata di consenso, quando serve, la apre
 * MainActivity e il risultato arriva in {@link #onConsentResult}.
 */
final class GoogleAuth {

    private static final String TAG = "RenameMusic";

    /**
     * Permessi: playlist di YouTube, email dell'account e cartella dati nascosta
     * dell'app su Drive (google.Scope/EmailScope/DriveAppDataScope in Go).
     */
    static final String YOUTUBE_SCOPE = "https://www.googleapis.com/auth/youtube";
    static final String EMAIL_SCOPE = "https://www.googleapis.com/auth/userinfo.email";
    static final String DRIVE_APPDATA_SCOPE = "https://www.googleapis.com/auth/drive.appdata";

    /** Attesa massima della risposta di Play Services (senza interazione). */
    private static final long AUTHORIZE_TIMEOUT_S = 60;
    /** Attesa massima della scelta dell'utente nella schermata di consenso. */
    private static final long CONSENT_TIMEOUT_MIN = 10;

    /** Activity in primo piano, che può aprire la schermata di consenso. */
    private static volatile MainActivity activity;
    /** Richiesta di consenso in attesa del risultato. */
    private static CompletableFuture<ActivityResult> pendingConsent;

    private GoogleAuth() {}

    static void attach(MainActivity a) {
        activity = a;
    }

    static void detach(MainActivity a) {
        if (activity == a) activity = null;
    }

    /** Risultato della schermata di consenso (registrato da MainActivity). */
    static void onConsentResult(ActivityResult result) {
        CompletableFuture<ActivityResult> f;
        synchronized (GoogleAuth.class) {
            f = pendingConsent;
            pendingConsent = null;
        }
        if (f != null) f.complete(result);
    }

    private static AuthorizationRequest request() {
        return AuthorizationRequest.builder()
            .setRequestedScopes(Arrays.asList(new Scope(YOUTUBE_SCOPE), new Scope(EMAIL_SCOPE), new Scope(DRIVE_APPDATA_SCOPE)))
            .build();
    }

    /**
     * Restituisce un access token come JSON {"token", "error", "signedOut"}.
     * Con interactive, se l'utente non ha ancora concesso il permesso, apre la
     * scelta dell'account e la schermata di consenso e ne attende l'esito; senza,
     * risponde signedOut. Non lancia mai (vedi GoHost).
     */
    static String token(Context context, boolean interactive) {
        try {
            AuthorizationClient client = Identity.getAuthorizationClient(context);
            AuthorizationResult result = Tasks.await(client.authorize(request()), AUTHORIZE_TIMEOUT_S, TimeUnit.SECONDS);
            if (result.hasResolution()) {
                if (!interactive) return result("", "", true);
                PendingIntent intent = result.getPendingIntent();
                if (intent == null) return result("", "Google Play Services non ha aperto la richiesta di accesso", false);
                ActivityResult consent = askConsent(intent);
                if (consent == null) return result("", "apri RenameMusic per completare l'accesso", false);
                if (consent.getResultCode() != Activity.RESULT_OK || consent.getData() == null) {
                    return result("", "accesso annullato", false);
                }
                result = client.getAuthorizationResultFromIntent(consent.getData());
            }
            String token = result.getAccessToken();
            // Nella schermata di consenso ogni permesso si può negare: senza
            // quello di YouTube l'accesso non serve a nulla.
            java.util.List<String> granted = result.getGrantedScopes();
            if (token == null || token.isEmpty() || (granted != null && !granted.contains(YOUTUBE_SCOPE))) {
                return result("", "non hai concesso l'accesso a YouTube: ricollega l'account e concedi il permesso «Gestisci il tuo account YouTube»", false);
            }
            return result(token, "", false);
        } catch (Throwable e) {
            Log.w(TAG, "accesso a Google fallito", e);
            return result("", describe(e), false);
        }
    }

    /**
     * Apre la schermata di consenso dall'Activity in primo piano e ne attende
     * l'esito; null se l'app non è in primo piano.
     */
    private static ActivityResult askConsent(PendingIntent intent) throws Exception {
        MainActivity a = activity;
        if (a == null) return null;
        CompletableFuture<ActivityResult> f = new CompletableFuture<>();
        synchronized (GoogleAuth.class) {
            if (pendingConsent != null) pendingConsent.cancel(false);
            pendingConsent = f;
        }
        a.runOnUiThread(() -> {
            try {
                a.launchGoogleConsent(intent);
            } catch (Throwable e) {
                f.completeExceptionally(e);
            }
        });
        return f.get(CONSENT_TIMEOUT_MIN, TimeUnit.MINUTES);
    }

    /** Toglie un access token dalla cache di Play Services. Non lancia mai. */
    static void clearToken(Context context, String token) {
        try {
            Tasks.await(
                Identity.getAuthorizationClient(context)
                    .clearToken(ClearTokenRequest.builder().setToken(token).build()),
                AUTHORIZE_TIMEOUT_S,
                TimeUnit.SECONDS
            );
        } catch (Throwable e) {
            Log.w(TAG, "rimozione del token di Google dalla cache fallita", e);
        }
    }

    /** Messaggio leggibile per gli errori più comuni di Play Services. */
    private static String describe(Throwable e) {
        Throwable cause = e instanceof ExecutionException && e.getCause() != null ? e.getCause() : e;
        if (cause instanceof ApiException) {
            int code = ((ApiException) cause).getStatusCode();
            switch (code) {
                case CommonStatusCodes.DEVELOPER_ERROR:
                    return "app non registrata su Google Cloud (manca il client OAuth Android con il nome del pacchetto e l'impronta SHA-1 di questa firma)";
                case CommonStatusCodes.NETWORK_ERROR:
                    return "nessuna connessione a Internet";
                case CommonStatusCodes.CANCELED:
                    return "accesso annullato";
                case CommonStatusCodes.SIGN_IN_REQUIRED:
                    return "nessun account Google sul telefono";
                default:
                    return "errore di Google Play Services (codice " + code + ")";
            }
        }
        if (cause instanceof java.util.concurrent.TimeoutException) {
            return "Google Play Services non ha risposto in tempo";
        }
        String msg = cause.getMessage();
        return msg != null && !msg.isEmpty() ? msg : cause.toString();
    }

    private static String result(String token, String error, boolean signedOut) {
        JSONObject out = new JSONObject();
        try {
            out.put("token", token);
            out.put("error", error);
            out.put("signedOut", signedOut);
        } catch (JSONException ignored) {
            // put con chiavi stringa non fallisce
        }
        return out.toString();
    }
}
