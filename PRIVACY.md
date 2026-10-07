# Informativa sulla privacy di RenameMusic

Ultimo aggiornamento: 1 ottobre 2026

RenameMusic è un'app per Windows e Android che rinomina i file musicali e ne scrive i tag. Facoltativamente può collegarsi al tuo account Google per gestire le tue playlist di YouTube. Questa pagina spiega quali dati usa l'app e come.

## In breve

- L'app **non ha server propri**: tutto avviene sul tuo dispositivo, oppure tra il tuo dispositivo e Google/YouTube.
- Lo sviluppatore **non riceve, non raccoglie e non conserva** alcun tuo dato.
- Nell'app non ci sono pubblicità, analisi statistiche né tracciamento.

## Dati dell'account Google

Se colleghi il tuo account Google (Impostazioni → Download → «Collega»), l'app chiede il permesso `https://www.googleapis.com/auth/youtube` e lo usa **solo** per queste operazioni, sempre avviate da te:

- leggere il nome del tuo canale YouTube, per mostrarti quale account è collegato;
- elencare le tue playlist, per importarle nell'app;
- leggere i video di una playlist e toglierne quelli appena scaricati, se attivi «Svuota dopo il download» su quella playlist;
- aggiungere a una tua playlist il video di cui hai incollato il link.

L'app non legge altri dati del tuo account, non pubblica nulla per tuo conto e non modifica altre playlist o altri contenuti.

## Dove sono conservati i dati

- **Windows**: il token di accesso fornito da Google è salvato sul tuo PC in `%AppData%\RenameMusic\google-token.dat`, cifrato con la protezione dati di Windows (DPAPI). Lo può leggere solo il tuo utente di Windows.
- **Android**: il permesso e i token sono gestiti da Google Play Services. L'app non li salva da sé.
- Nei file di configurazione dell'app restano solo il fatto che l'account è collegato e il nome del canale.

Questi dati non vengono mai inviati allo sviluppatore né a terzi. Viaggiano solo verso i server di Google, quando l'app chiama la YouTube Data API.

## Revocare l'accesso

- Nell'app: Impostazioni → Download → «Scollega». L'app revoca il permesso anche su Google ed elimina il token salvato.
- Dal tuo account Google: <https://myaccount.google.com/permissions>.

Anche disinstallare l'app elimina i dati salvati sul dispositivo.

## Rispetto delle norme di Google

L'uso e il trasferimento ad altre app delle informazioni ricevute dalle API di Google rispettano le [Norme sui dati utente dei servizi API di Google](https://developers.google.com/terms/api-services-user-data-policy), compresi i requisiti di uso limitato (Limited Use).

## Contatti

Per domande su questa informativa apri una segnalazione su <https://github.com/SimoxSpeed/RenameMusic/issues>.
