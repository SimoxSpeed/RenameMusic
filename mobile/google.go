package mobile

import (
	"context"
	"encoding/json"
	"errors"

	"renamemusic/internal/google"
)

// androidGoogle implementa core.GoogleAuth sopra Google Play Services
// (AuthorizationClient, lato Java): è Play Services a conservare il permesso
// concesso e a rinnovare l'access token, quindi qui non si salva nulla. Il
// client OAuth Android del progetto Google Cloud riconosce l'app dal nome del
// pacchetto e dall'impronta SHA-1 della firma: nessuna credenziale nel codice.
type androidGoogle struct {
	host Host
}

// tokenResult è la risposta JSON di Host.GoogleToken.
type tokenResult struct {
	Token     string `json:"token"`
	Error     string `json:"error"`
	SignedOut bool   `json:"signedOut"`
}

func (androidGoogle) Configured() bool { return true }

// token chiama Host.GoogleToken (bloccante) senza bloccare oltre ctx: la
// schermata di Play Services resta aperta, ma il core non la aspetta più.
func (g androidGoogle) token(ctx context.Context, interactive bool) (string, error) {
	done := make(chan tokenResult, 1)
	go func() {
		var r tokenResult
		if err := json.Unmarshal([]byte(g.host.GoogleToken(interactive)), &r); err != nil {
			r.Error = "risposta non valida da Google Play Services"
		}
		done <- r
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-done:
		switch {
		case r.SignedOut:
			return "", google.ErrSignedOut
		case r.Error != "":
			return "", errors.New(r.Error)
		case r.Token == "":
			return "", errors.New("Google Play Services non ha restituito un token")
		}
		return r.Token, nil
	}
}

func (g androidGoogle) SignIn(ctx context.Context) error {
	_, err := g.token(ctx, true)
	return err
}

func (g androidGoogle) Token(ctx context.Context) (string, error) {
	return g.token(ctx, false)
}

func (g androidGoogle) Invalidate(token string) {
	g.host.GoogleClearToken(token)
}

// SignOut revoca su Google il permesso (con l'access token corrente, che
// revoca l'intero consenso) e lo toglie dalla cache di Play Services.
func (g androidGoogle) SignOut(ctx context.Context) error {
	token, err := g.token(ctx, false)
	if err != nil {
		return err
	}
	g.host.GoogleClearToken(token)
	return google.Revoke(ctx, token)
}
