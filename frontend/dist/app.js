const folder = document.getElementById("folder");
const folderLabel = document.getElementById("folderLabel");
const statusEl = document.getElementById("status");
const filesEl = document.getElementById("files");
const logsEl = document.getElementById("logs");
const countEl = document.getElementById("count");
const mp3CountEl = document.getElementById("mp3Count");

function api() {
	if (!window.go || !window.go.main || !window.go.main.App) {
		throw new Error("Backend Wails non disponibile");
	}
	return window.go.main.App;
}

function setStatus(message, ok) {
	statusEl.textContent = message || "";
	statusEl.className = "status " + (message ? (ok ? "ok" : "err") : "");
}

function render(state) {
	const files = state.files || [];
	const mp3Count = files.filter(file => file.mp3).length;

	folder.value = state.folder || "";
	folderLabel.textContent = state.folder || "";
	countEl.textContent = files.length + " file";
	mp3CountEl.textContent = mp3Count + " MP3";

	if (files.length === 0) {
		filesEl.innerHTML = '<div class="empty">Nessun file scansionato.</div>';
	} else {
		filesEl.innerHTML = [
			"<table>",
			"<thead><tr><th>File attuale</th><th>Anteprima nuovo nome</th><th>Tag</th></tr></thead>",
			"<tbody>",
			files.map(file => (
				"<tr><td>" + esc(file.name) + "</td><td>" + esc(file.preview) + "</td><td>" +
				(file.mp3 ? '<span class="badge">MP3</span>' : "") +
				"</td></tr>"
			)).join(""),
			"</tbody></table>",
		].join("");
	}

	const logs = state.logs || [];
	logsEl.innerHTML = logs.length
		? logs.map(log => "<li>" + esc(log) + "</li>").join("")
		: "<li>Nessuna attivita.</li>";
}

function esc(value) {
	return String(value).replace(/[&<>"']/g, ch => ({
		"&": "&amp;",
		"<": "&lt;",
		">": "&gt;",
		'"': "&quot;",
		"'": "&#39;",
	}[ch]));
}

async function run(action) {
	setStatus("Operazione in corso...", true);
	try {
		const response = await action();
		render(response.state);
		setStatus(response.message, response.ok);
	} catch (err) {
		setStatus("Errore: " + err.message, false);
	}
}

document.getElementById("selectFolder").addEventListener("click", () => run(() => api().SelectFolder()));
document.getElementById("saveFolder").addEventListener("click", () => run(() => api().SetFolder(folder.value)));
document.getElementById("scan").addEventListener("click", () => run(() => api().Scan()));
document.getElementById("rename").addEventListener("click", () => run(() => api().RenameFiles()));
document.getElementById("tags").addEventListener("click", () => run(() => api().WriteTags()));

run(() => api().GetState());
