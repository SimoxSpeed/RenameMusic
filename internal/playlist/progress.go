package playlist

// Fasi riportate da Progress durante l'installazione di yt-dlp/ffmpeg.
const (
	PhaseDownload = "download" // download dal web (byte ricevuti)
	PhaseExtract  = "extract"  // estrazione dallo zip (byte estratti, solo ffmpeg)
)

// Progress riceve l'avanzamento di un'installazione: fase (PhaseDownload o
// PhaseExtract), byte completati e totali (total <= 0 se il server non ha
// indicato la dimensione). Può essere nil.
type Progress func(phase string, done, total int64)

// progressWriter conta i byte che gli passano attraverso (io.TeeReader) e
// notifica Progress in modo diradato: a ogni punto percentuale in più, oppure
// ogni MB se il totale non è noto, così la UI non riceve un evento per ogni
// blocco letto.
type progressWriter struct {
	fn    Progress
	phase string
	total int64

	done       int64
	lastPct    int64
	lastReport int64
}

// newProgressWriter crea il contatore e notifica subito lo 0%, così la UI
// mostra la barra appena inizia la fase.
func newProgressWriter(fn Progress, phase string, total int64) *progressWriter {
	w := &progressWriter{fn: fn, phase: phase, total: total, lastPct: -1}
	w.report()
	return w
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	w.report()
	return len(p), nil
}

func (w *progressWriter) report() {
	if w.fn == nil {
		return
	}
	if w.total > 0 {
		pct := w.done * 100 / w.total
		if pct == w.lastPct {
			return
		}
		w.lastPct = pct
	} else if w.done != 0 && w.done-w.lastReport < 1<<20 {
		return
	}
	w.lastReport = w.done
	w.fn(w.phase, w.done, w.total)
}
