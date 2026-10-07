//go:build !windows

package google

// Fuori da Windows (test, Android) il file del token resta in chiaro: su
// Android DesktopAuth non si usa (l'accesso passa da Google Play Services), e
// il file sta comunque nella cartella privata dell'app con permessi 0600.
func protect(data []byte) ([]byte, error)   { return data, nil }
func unprotect(data []byte) ([]byte, error) { return data, nil }
