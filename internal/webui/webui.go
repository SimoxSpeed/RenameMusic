package webui

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	appfs "renamemusic/internal/fs"
	"renamemusic/internal/parser"
	"renamemusic/internal/rename"
	"renamemusic/internal/rules"
)

type appState struct {
	mu      sync.Mutex
	folder  string
	scanned []string
	logs    []string
}

type fileView struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Preview string `json:"preview"`
	MP3     bool   `json:"mp3"`
}

type resultView struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
	Tagged  bool   `json:"tagged"`
}

type stateResponse struct {
	Folder string     `json:"folder"`
	Files  []fileView `json:"files"`
	Logs   []string   `json:"logs"`
}

type actionResponse struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	State   stateResponse `json:"state"`
	Results []resultView  `json:"results,omitempty"`
}

type folderRequest struct {
	Path string `json:"path"`
}

func Run() error {
	state := &appState{
		folder: rules.DefaultStartFolder,
		logs:   []string{"Interfaccia pronta."},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/api/state", state.handleState)
	mux.HandleFunc("/api/folder", state.handleFolder)
	mux.HandleFunc("/api/choose", state.handleChooseFolder)
	mux.HandleFunc("/api/scan", state.handleScan)
	mux.HandleFunc("/api/rename", state.handleRename)
	mux.HandleFunc("/api/tags", state.handleTags)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	url := "http://" + listener.Addr().String()
	fmt.Println("Rename Music Web UI:", url)
	fmt.Println("Usa --cli per avviare il vecchio menu testuale.")
	openBrowser(url)

	return http.Serve(listener, mux)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pageTemplate.Execute(w, nil)
}

func (s *appState) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, actionResponse{OK: true, State: s.snapshotLocked()})
}

func (s *appState) handleFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req folderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, actionResponse{OK: false, Message: "Percorso non valido.", State: s.snapshotLocked()})
		return
	}

	path := strings.Trim(req.Path, `" `)
	if !appfs.IsDir(path) {
		writeJSON(w, actionResponse{OK: false, Message: "La cartella indicata non esiste.", State: s.snapshotLocked()})
		return
	}

	s.mu.Lock()
	s.folder = path
	s.scanned = nil
	s.addLogLocked("Cartella selezionata: " + path)
	state := s.snapshot()
	s.mu.Unlock()

	writeJSON(w, actionResponse{OK: true, Message: "Cartella selezionata.", State: state})
}

func (s *appState) handleChooseFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	path, err := chooseFolder()
	if err != nil {
		writeJSON(w, actionResponse{OK: false, Message: err.Error(), State: s.snapshotLocked()})
		return
	}
	if path == "" {
		writeJSON(w, actionResponse{OK: false, Message: "Selezione annullata.", State: s.snapshotLocked()})
		return
	}

	s.mu.Lock()
	s.folder = path
	s.scanned = nil
	s.addLogLocked("Cartella selezionata: " + path)
	state := s.snapshot()
	s.mu.Unlock()

	writeJSON(w, actionResponse{OK: true, Message: "Cartella selezionata.", State: state})
}

func (s *appState) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.mu.Lock()
	folder := s.folder
	s.mu.Unlock()

	files, err := rename.NewService(folder).Scan()
	if err != nil {
		writeJSON(w, actionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: s.snapshotLocked()})
		return
	}

	s.mu.Lock()
	s.scanned = files
	s.addLogLocked(fmt.Sprintf("Scansione completata: %d file audio.", len(files)))
	state := s.snapshot()
	s.mu.Unlock()

	writeJSON(w, actionResponse{OK: true, Message: "Scansione completata.", State: state})
}

func (s *appState) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.mu.Lock()
	folder := s.folder
	files := append([]string(nil), s.scanned...)
	s.mu.Unlock()

	service := rename.NewService(folder)
	if files == nil {
		var err error
		files, err = service.Scan()
		if err != nil {
			writeJSON(w, actionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: s.snapshotLocked()})
			return
		}
	}

	results, err := service.RenameAll(files)
	if err != nil {
		writeJSON(w, actionResponse{OK: false, Message: "Errore rinomina: " + err.Error(), State: s.snapshotLocked()})
		return
	}

	views := make([]resultView, 0, len(results))
	for _, result := range results {
		views = append(views, resultView{
			OldName: filepath.Base(result.OldPath),
			NewName: result.NewName,
			Tagged:  result.Tagged,
		})
	}

	s.mu.Lock()
	s.scanned = nil
	s.addLogLocked(fmt.Sprintf("Rinomina completata: %d file elaborati.", len(results)))
	state := s.snapshot()
	s.mu.Unlock()

	writeJSON(w, actionResponse{OK: true, Message: "Rinomina completata.", State: state, Results: views})
}

func (s *appState) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.mu.Lock()
	folder := s.folder
	files := append([]string(nil), s.scanned...)
	s.mu.Unlock()

	if files == nil {
		var err error
		files, err = rename.NewService(folder).Scan()
		if err != nil {
			writeJSON(w, actionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: s.snapshotLocked()})
			return
		}
	}

	written, err := rename.WriteTagsAll(files)
	if err != nil {
		writeJSON(w, actionResponse{OK: false, Message: "Errore scrittura tag: " + err.Error(), State: s.snapshotLocked()})
		return
	}

	s.mu.Lock()
	s.addLogLocked(fmt.Sprintf("Tag scritti su %d file MP3.", written))
	state := s.snapshot()
	s.mu.Unlock()

	writeJSON(w, actionResponse{OK: true, Message: "Tag scritti.", State: state})
}

func (s *appState) snapshotLocked() stateResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot()
}

func (s *appState) snapshot() stateResponse {
	files := make([]fileView, 0, len(s.scanned))
	for _, path := range s.scanned {
		name := filepath.Base(path)
		ext := parser.Extension(name)
		preview := rules.NormalizeFileBase(parser.RemoveExtension(name)) + "." + ext
		files = append(files, fileView{
			Name:    name,
			Path:    path,
			Preview: preview,
			MP3:     ext == "mp3",
		})
	}
	logs := append([]string(nil), s.logs...)
	return stateResponse{Folder: s.folder, Files: files, Logs: logs}
}

func (s *appState) addLogLocked(message string) {
	s.logs = append([]string{message}, s.logs...)
	if len(s.logs) > 12 {
		s.logs = s.logs[:12]
	}
}

func chooseFolder() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("il selettore cartella automatico e' disponibile solo su Windows; inserisci il percorso a mano")
	}

	script := `[Console]::OutputEncoding = [Text.UTF8Encoding]::UTF8; Add-Type -AssemblyName System.Windows.Forms; $dialog = New-Object System.Windows.Forms.FolderBrowserDialog; $dialog.Description = 'Seleziona cartella Rename Music'; $dialog.ShowNewFolderButton = $false; if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $dialog.SelectedPath }`
	out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script).Output()
	if err != nil {
		return "", fmt.Errorf("impossibile aprire il selettore cartella")
	}
	return strings.TrimSpace(string(out)), nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func writeJSON(w http.ResponseWriter, response actionResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
}

func methodNotAllowed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	_ = json.NewEncoder(w).Encode(actionResponse{OK: false, Message: "Metodo non consentito."})
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="it">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Rename Music</title>
	<style>
		:root {
			color-scheme: light;
			--bg: #f6f7f9;
			--panel: #ffffff;
			--line: #d9dee7;
			--text: #172033;
			--muted: #667085;
			--primary: #176b87;
			--primary-strong: #0f5268;
			--danger: #a43b3b;
			--ok: #2d7d46;
		}
		* { box-sizing: border-box; }
		body {
			margin: 0;
			background: var(--bg);
			color: var(--text);
			font: 14px/1.4 "Segoe UI", system-ui, sans-serif;
		}
		header {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: 16px;
			padding: 16px 24px;
			background: #1f2937;
			color: white;
		}
		h1 {
			margin: 0;
			font-size: 20px;
			font-weight: 650;
		}
		main {
			max-width: 1180px;
			margin: 0 auto;
			padding: 20px 24px 32px;
		}
		.toolbar {
			display: grid;
			grid-template-columns: minmax(0, 1fr) auto auto;
			gap: 10px;
			align-items: center;
			margin-bottom: 16px;
		}
		input {
			width: 100%;
			min-height: 38px;
			border: 1px solid var(--line);
			border-radius: 6px;
			padding: 8px 10px;
			color: var(--text);
			background: white;
		}
		button {
			min-height: 38px;
			border: 1px solid var(--line);
			border-radius: 6px;
			padding: 8px 12px;
			background: white;
			color: var(--text);
			cursor: pointer;
			font-weight: 600;
			white-space: nowrap;
		}
		button.primary {
			border-color: var(--primary);
			background: var(--primary);
			color: white;
		}
		button.primary:hover { background: var(--primary-strong); }
		button:hover { border-color: #aab3c2; }
		.actions {
			display: flex;
			flex-wrap: wrap;
			gap: 10px;
			margin-bottom: 16px;
		}
		.status {
			min-height: 32px;
			margin-bottom: 12px;
			color: var(--muted);
		}
		.status.ok { color: var(--ok); }
		.status.err { color: var(--danger); }
		.grid {
			display: grid;
			grid-template-columns: minmax(0, 1fr) 320px;
			gap: 16px;
			align-items: start;
		}
		section {
			background: var(--panel);
			border: 1px solid var(--line);
			border-radius: 8px;
			overflow: hidden;
		}
		section h2 {
			margin: 0;
			padding: 12px 14px;
			border-bottom: 1px solid var(--line);
			font-size: 15px;
			background: #fbfcfe;
		}
		table {
			width: 100%;
			border-collapse: collapse;
			table-layout: fixed;
		}
		th, td {
			padding: 10px 12px;
			border-bottom: 1px solid #edf0f5;
			text-align: left;
			vertical-align: top;
			word-break: break-word;
		}
		th {
			color: var(--muted);
			font-size: 12px;
			text-transform: uppercase;
			letter-spacing: 0;
			background: #fbfcfe;
		}
		.empty {
			padding: 22px 14px;
			color: var(--muted);
		}
		.badge {
			display: inline-block;
			border: 1px solid #bdd7c8;
			color: #24683c;
			border-radius: 999px;
			padding: 1px 7px;
			font-size: 12px;
			font-weight: 650;
		}
		.log {
			padding: 12px 14px;
			margin: 0;
			list-style: none;
			color: var(--muted);
		}
		.log li {
			padding: 8px 0;
			border-bottom: 1px solid #edf0f5;
		}
		.log li:last-child { border-bottom: 0; }
		@media (max-width: 820px) {
			header { padding: 14px 16px; }
			main { padding: 16px; }
			.toolbar { grid-template-columns: 1fr; }
			.grid { grid-template-columns: 1fr; }
			button { width: 100%; }
		}
	</style>
</head>
<body>
	<header>
		<h1>Rename Music</h1>
		<div id="count">0 file</div>
	</header>
	<main>
		<div class="toolbar">
			<input id="folder" type="text" aria-label="Cartella" placeholder="Percorso cartella">
			<button id="saveFolder">Usa percorso</button>
			<button id="chooseFolder">Scegli cartella</button>
		</div>
		<div class="actions">
			<button id="scan" class="primary">Avvia scansione</button>
			<button id="rename">Rinomina file</button>
			<button id="tags">Scrivi tag</button>
		</div>
		<div id="status" class="status"></div>
		<div class="grid">
			<section>
				<h2>File audio</h2>
				<div id="files"></div>
			</section>
			<section>
				<h2>Attivita'</h2>
				<ul id="logs" class="log"></ul>
			</section>
		</div>
	</main>
	<script>
		const folder = document.getElementById('folder');
		const statusEl = document.getElementById('status');
		const filesEl = document.getElementById('files');
		const logsEl = document.getElementById('logs');
		const countEl = document.getElementById('count');

		async function api(path, body) {
			const options = body === undefined
				? {}
				: { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
			const res = await fetch(path, options);
			return await res.json();
		}

		function setStatus(message, ok) {
			statusEl.textContent = message || '';
			statusEl.className = 'status ' + (message ? (ok ? 'ok' : 'err') : '');
		}

		function render(state) {
			folder.value = state.folder || '';
			countEl.textContent = (state.files ? state.files.length : 0) + ' file';
			const files = state.files || [];
			if (files.length === 0) {
				filesEl.innerHTML = '<div class="empty">Nessun file scansionato.</div>';
			} else {
				filesEl.innerHTML = '<table><thead><tr><th>File attuale</th><th>Anteprima nuovo nome</th><th>Tag</th></tr></thead><tbody>' +
					files.map(file => '<tr><td>' + esc(file.name) + '</td><td>' + esc(file.preview) + '</td><td>' + (file.mp3 ? '<span class="badge">MP3</span>' : '') + '</td></tr>').join('') +
					'</tbody></table>';
			}
			const logs = state.logs || [];
			logsEl.innerHTML = logs.length
				? logs.map(log => '<li>' + esc(log) + '</li>').join('')
				: '<li>Nessuna attivita.</li>';
		}

		function esc(value) {
			return String(value).replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
		}

		async function run(path, body) {
			setStatus('Operazione in corso...', true);
			try {
				const data = await api(path, body);
				render(data.state);
				setStatus(data.message, data.ok);
			} catch (err) {
				setStatus('Errore: ' + err.message, false);
			}
		}

		document.getElementById('saveFolder').addEventListener('click', () => run('/api/folder', { path: folder.value }));
		document.getElementById('chooseFolder').addEventListener('click', () => run('/api/choose', {}));
		document.getElementById('scan').addEventListener('click', () => run('/api/scan', {}));
		document.getElementById('rename').addEventListener('click', () => run('/api/rename', {}));
		document.getElementById('tags').addEventListener('click', () => run('/api/tags', {}));
		api('/api/state').then(data => render(data.state));
	</script>
</body>
</html>`))
