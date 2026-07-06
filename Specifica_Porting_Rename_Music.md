# Specifica di Porting - Rename Music (Java → Go)

## Obiettivo

Realizzare un porting funzionalmente equivalente dell'applicazione Java
in Go, eliminando JavaFX e sostituendo l'interfaccia grafica con una
CLI.

## Architettura

-   `main.go` come entrypoint.
-   Package separati:
    -   `cmd/` menu CLI.
    -   `internal/rename` logica di rinomina.
    -   `internal/tags` gestione ID3.
    -   `internal/resources` costanti e regole.
    -   `internal/files` scansione filesystem.

## Funzionalità da portare

-   Selezione directory.
-   Scansione file audio.
-   Normalizzazione nomi file.
-   Applicazione delle regole di sostituzione presenti nelle risorse.
-   Uniformazione `feat.` → `ft`.
-   `(VIP)` → `VIP`.
-   Rimozione contenuti fra parentesi quadre.
-   `X` → `x`.
-   `Re-Crank` → `Remix`.
-   `tha Supreme` → `thasup`.
-   `Prod.` → `prod.`.
-   Eliminazione spazi multipli.
-   Rinomina fisica dei file.
-   Parsing del nome per estrarre Artist e Title.
-   Gestione Remix, VIP, Bootleg, Flip, Mashup, Cover e varianti.
-   Scrittura tag ID3 (Title, Artist).

## Requisiti

-   Singolo eseguibile (`go build`).
-   Nessuna dipendenza da Java.
-   Comportamento equivalente al progetto originale.
