---
paths:
  - "internal/musicbrainz/**"
  - "internal/core/musicbrainz.go"
  - "frontend/src/dialogs/TagPromptDialog.tsx"
  - "frontend/src/lib/prompts.ts"
---

# Proposte da MusicBrainz

Ricerca su MusicBrainz dei nomi proposti nel popup delle tracce da confermare.

- **`musicbrainz`** — client REST minimo della ricerca di MusicBrainz (registrazioni): `Lookup` (nome senza separatore: stesse parole cercate in titolo e artista, poi il nome come titolo esatto, cioè i brani con quel titolo di artisti diversi, poi l'artista con cui il nome inizia o finisce più il resto come titolo; ci si ferma al primo passo che trova), `LookupFields` (titolo e artista già dedotti). Restituiscono tutti i brani trovati (una ricerca chiede fino a 100 registrazioni, il massimo per richiesta): le registrazioni dello stesso brano si riuniscono e si ordinano per pertinenza, poi per diffusione (numero di uscite, `rank`), dopo aver scartato quelle poco attendibili (`Matches`, `SameTitle`, `Similar`). `Recording.Base` dà il nome `Artisti - Titolo ft Ospiti`. Una richiesta al secondo per processo (`MinInterval`) e User-Agent `RenameMusic/<versione>`, come chiede MusicBrainz.
- **Proposta da MusicBrainz** (`rules.Config.MusicBrainz`, default `true`, interruttore in Impostazioni → Generale; segue regole/predefiniti/sincronizzazione come `SimpleMode`): il popup di una traccia da confermare (tag sconosciuti o da rivedere) prima mostra solo l'attesa della ricerca (`SuggestTrackNames`, con «Salta ricerca»), poi il campo: col primo nome trovato, gli altri brani in un pannello sotto il campo, con una casella per cercare fra i risultati (`promptFilter`, senza maiuscole né accenti; Invio sceglie il primo) e un elenco da toccare (`.tag-prompt-results`, per esempio quando il nome ha solo il titolo) e il link per tornare al nome di prima; se la ricerca non riesce o non trova nulla, il campo ha il nome di prima e una riga ne dice il motivo (`TrackSuggestions.Error`). Finita la ricerca della traccia mostrata parte quella in anticipo per la successiva (una per traccia, `suggestionsRef`). Il popup resta: niente viene convertito senza conferma. Il core tiene in cache i risultati (`App.suggestions`, non gli errori) e scarta i nomi uguali a quello già proposto; gli errori non vanno nel registro.
