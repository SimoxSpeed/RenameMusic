package playlist

import (
	"os/exec"
	"syscall"
)

// hideWindow evita che ogni processo yt-dlp.exe apra un'effimera finestra di
// console: l'app è una GUI e i download avvengono in background.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
