//go:build !windows

package playlist

import "os/exec"

// hideWindow non ha effetto fuori da Windows: solo lì ogni processo figlio di
// una GUI aprirebbe una finestra di console.
func hideWindow(_ *exec.Cmd) {}
