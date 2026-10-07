// InstallProgress è il payload dell'evento install:progress (download di
// yt-dlp/ffmpeg ed estrazione di ffmpeg). total <= 0 se la dimensione non è nota.
export type InstallProgress = { tool: string; phase: 'download' | 'extract'; done: number; total: number }

// installPercent restituisce la percentuale intera, o null se il totale non è noto.
export function installPercent(p: InstallProgress): number | null {
    if (p.total <= 0) return null
    return Math.min(100, Math.floor((p.done / p.total) * 100))
}

export function formatMB(bytes: number): string {
    return (bytes / (1024 * 1024)).toLocaleString('it-IT', { minimumFractionDigits: 1, maximumFractionDigits: 1 })
}

// installLabel descrive l'avanzamento, es. "Download di ffmpeg · 45% (90,1 / 199,3 MB)".
export function installLabel(p: InstallProgress): string {
    const pct = installPercent(p)
    if (p.phase === 'extract') {
        return `Estrazione di ${p.tool}` + (pct !== null ? ` · ${pct}%` : '…')
    }
    if (pct === null) return `Download di ${p.tool} · ${formatMB(p.done)} MB`
    return `Download di ${p.tool} · ${pct}% (${formatMB(p.done)} / ${formatMB(p.total)} MB)`
}
