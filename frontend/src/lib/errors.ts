// errorMessage: testo di un errore arrivato da una promessa rifiutata o da un
// catch (un Error, l'errore di un plugin con message, o una stringa del core).
export function errorMessage(err: unknown): string {
    return (err as { message?: string } | null)?.message ?? String(err)
}
