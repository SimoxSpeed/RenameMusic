//go:build tools

package mobile

// Il codice generato da `gomobile bind` importa golang.org/x/mobile/bind, che
// quindi deve restare in go.mod anche se nessun sorgente del modulo lo usa:
// questo import (escluso dalle build normali dal tag "tools") impedisce a
// `go mod tidy` di rimuoverlo.
import _ "golang.org/x/mobile/bind"
