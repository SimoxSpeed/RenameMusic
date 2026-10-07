package google

import "os"

// Credenziali del client OAuth "App desktop" del progetto Google Cloud, usate
// solo su desktop (su Android l'accesso passa da Google Play Services, che
// riconosce l'app dal nome del pacchetto e dall'impronta della firma).
//
// Non sono nel repository: le imposta credentials_local.go (ignorato da git,
// vedi credentials_local.go.example), compilato insieme al resto quando c'è.
// Per le app installate Google non considera segreto il client secret, ma
// tenerlo fuori dal repository pubblico evita che venga riusato altrove.
// RENAMEMUSIC_GOOGLE_CLIENT_ID / RENAMEMUSIC_GOOGLE_CLIENT_SECRET, se
// valorizzate, hanno la precedenza (utile in sviluppo).
var clientID, clientSecret string

// credentials restituisce le credenziali in uso ("" se non configurate).
func credentials() (id, secret string) {
	id, secret = clientID, clientSecret
	if v := os.Getenv("RENAMEMUSIC_GOOGLE_CLIENT_ID"); v != "" {
		id = v
		secret = os.Getenv("RENAMEMUSIC_GOOGLE_CLIENT_SECRET")
	}
	return id, secret
}
