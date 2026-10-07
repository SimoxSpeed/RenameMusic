<#
.SYNOPSIS
    Pubblica una nuova versione di RenameMusic: versione, build, commit, tag e push.

.DESCRIPTION
    1. controlli: branch, credenziali di Google per l'exe
       (internal\google\credentials_local.go, obbligatorio: le variabili
       d'ambiente non bastano), keystore per gli APK
       (frontend\android\keystore.properties), tag libero (in locale e su origin), modifiche
       pendenti e GitHub CLI. Se il tag esiste già chiede se ricrearlo
       (sostituito solo al push, insieme a un'eventuale release GitHub
       rimasta su quel tag) o annullare. Con gh pronta apre nel Blocco note un
       file temporaneo vuoto in cui scrivere le note di rilascio: letto il
       testo, il file si elimina (le note restano solo in memoria)
    2. incrementa la versione in internal/update/update.go (update.Version,
       unica fonte della versione: la usano il controllo aggiornamenti e l'APK)
    3. compila l'exe desktop (wails build)
    4. compila gli APK (mobile\build-apk.ps1)
       Prima del commit verifica che l'exe contenga il client ID di Google e
       che gli APK siano firmati con il certificato delle versioni pubblicate
       (mobile\signing.ps1).
    5. commit di tutte le modifiche (dopo conferma) con il messaggio fisso
       "chore(release): 🚀 release <versione>", tag annotato v<versione> e push
       di branch e tag insieme (--atomic: o tutti e due o nessuno)
    6. crea la release su GitHub con GitHub CLI (gh): titolo "RenameMusic
       <versione>", le note scritte al passo 1 ed exe + APK allegati.
       Senza gh (o se non è autenticata) apre la pagina della nuova release,
       da completare a mano.

    Se qualcosa fallisce, o alla conferma rispondi no, non viene committato
    nulla e lo script annulla da solo le sue modifiche (versione, staging,
    exe e APK in build\bin, che tornano quelli di prima delle build).
    Se fallisce il push spiega come riprovare o tornare indietro.

.PARAMETER Bump
    Obbligatorio. Parte della versione da incrementare (semver):
    patch (1.2.3 -> 1.2.4), minor (1.2.3 -> 1.3.0), major (1.2.3 -> 2.0.0).
    Accettato anche nella forma -patch / --patch.

.PARAMETER NoBuild
    Salta le build (solo se exe e APK in build\bin sono già stati compilati
    con la nuova versione).

.PARAMETER JavaHome
    JDK 21 da passare a build-apk.ps1.

.PARAMETER Yes
    Non chiede conferma prima di commit e push. Con -Yes un tag già
    esistente interrompe la release (non viene mai ricreato senza chiedere).

.EXAMPLE
    npm run release -- minor

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File release.ps1 patch -NoBuild
#>
param(
    [Parameter(Position = 0)]
    [string]$Bump = '',
    # Forme alternative (-patch, --patch): PowerShell le lega a questi switch.
    [switch]$Patch,
    [switch]$Minor,
    [switch]$Major,
    [switch]$NoBuild,
    [string]$JavaHome = '',
    [switch]$Yes,
    # Argomenti in più (es. due tipi di release, o un'opzione sconosciuta):
    # raccolti qui per segnalarli con un messaggio chiaro.
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Rest
)

# 'Continue' come in build-apk.ps1: git, wails e Gradle scrivono su stderr anche
# messaggi normali. Gli errori veri si intercettano con l'exit code.
$ErrorActionPreference = 'Continue'

$root = $PSScriptRoot
$versionFile = Join-Path $root 'internal\update\update.go'
$versionFileRel = 'internal\update\update.go'
$repoSlug = 'SimoxSpeed/RenameMusic'
$repoUrl = "https://github.com/$repoSlug"
$desktopAsset = 'build\bin\RenameMusic.exe'
$apkAssets = @('build\bin\RenameMusic-arm64-v8a.apk', 'build\bin\RenameMusic-x86_64.apk')
$allAssets = @($desktopAsset) + $apkAssets
$credentialsFileRel = 'internal\google\credentials_local.go'
$credentialsFile = Join-Path $root $credentialsFileRel
$googleClientID = ''
# Firma degli APK: keystore.properties e certificato atteso.
. (Join-Path $root 'mobile\signing.ps1')
# Copia di exe e APK di prima delle build, da rimettere a posto se la release
# non va in porto (build\bin è ignorata da git).
$backupDir = Join-Path $root 'build\bin\.release-backup'
$utf8 = New-Object System.Text.UTF8Encoding $false
$steps = 6
$width = 68
$clock = [System.Diagnostics.Stopwatch]::StartNew()

# ---- Output ---------------------------------------------------------------------

function Badge([string]$text, [ConsoleColor]$fg, [ConsoleColor]$bg) {
    Write-Host " $text " -ForegroundColor $fg -BackgroundColor $bg -NoNewline
}

function Line([string]$char, [ConsoleColor]$color) { Write-Host ($char * $width) -ForegroundColor $color }

function Step([int]$n, [string]$title) {
    Write-Host ''
    Badge "$n/$steps" Black Cyan
    Write-Host " $title" -ForegroundColor Cyan
    Line '─' DarkGray
}

function Ok([string]$msg) { Write-Host '  ' -NoNewline; Badge 'OK' Black Green; Write-Host " $msg" }
function Warn([string]$msg) { Write-Host '  ' -NoNewline; Badge '!!' Black Yellow; Write-Host " $msg" -ForegroundColor Yellow }
function Skip([string]$msg) { Write-Host '  ' -NoNewline; Badge '--' Black DarkGray; Write-Host " $msg" -ForegroundColor DarkGray }
function Detail([string]$msg) { Write-Host "       $msg" -ForegroundColor DarkGray }

function Banner([string]$badge, [ConsoleColor]$fg, [ConsoleColor]$bg, [ConsoleColor]$color, [string]$title) {
    Write-Host ''
    Line '═' $color
    Write-Host '  ' -NoNewline
    Badge $badge $fg $bg
    Write-Host " $title" -ForegroundColor $color
    Line '═' $color
}

function Elapsed {
    $t = $clock.Elapsed
    if ($t.TotalMinutes -ge 1) { return '{0}m {1:00}s' -f [int][math]::Floor($t.TotalMinutes), $t.Seconds }
    return '{0}s' -f [int]$t.TotalSeconds
}

# Mostra come si usa lo script, con le versioni che otterresti a partire da
# quella attuale.
function Show-Usage([string]$current) {
    Write-Host ''
    Write-Host '  Uso' -ForegroundColor White
    Write-Host '    npm run release -- ' -NoNewline -ForegroundColor Gray
    Write-Host '<patch|minor|major>' -NoNewline -ForegroundColor Yellow
    Write-Host ' [-NoBuild] [-Yes] [-JavaHome <jdk>]' -ForegroundColor DarkGray
    Write-Host '    powershell -ExecutionPolicy Bypass -File release.ps1 ' -NoNewline -ForegroundColor Gray
    Write-Host '<patch|minor|major>' -ForegroundColor Yellow
    if ($current) {
        Write-Host ''
        Write-Host '  Versione attuale: ' -NoNewline -ForegroundColor White
        Write-Host $current -ForegroundColor Cyan
        $choices = @(
            @('patch', 'correzioni di bug'),
            @('minor', 'nuove funzionalità'),
            @('major', 'cambiamenti importanti')
        )
        foreach ($c in $choices) {
            Write-Host ('    {0,-7}' -f $c[0]) -NoNewline -ForegroundColor Yellow
            Write-Host ' -> ' -NoNewline -ForegroundColor DarkGray
            Write-Host ('{0,-9}' -f (Get-NextVersion $current $c[0])) -NoNewline -ForegroundColor Green
            Write-Host $c[1] -ForegroundColor DarkGray
        }
    }
    Write-Host ''
}

function Fail-Usage([string]$msg, [string]$current) {
    Banner 'ERRORE' White DarkRed Red $msg
    Show-Usage $current
    exit 1
}

# ---- Comandi ----------------------------------------------------------------------

function Get-NextVersion([string]$current, [string]$kind) {
    $v = [version]$current
    switch ($kind) {
        'major' { return "$($v.Major + 1).0.0" }
        'minor' { return "$($v.Major).$($v.Minor + 1).0" }
        'patch' { return "$($v.Major).$($v.Minor).$($v.Build + 1)" }
    }
}

# Esegue un comando lungo lasciando scorrere il suo output (build).
function Invoke-Checked([string]$what, [scriptblock]$block) {
    & $block
    if ($LASTEXITCODE -ne 0) { throw "$what fallito (exit code $LASTEXITCODE)" }
}

# Esegue un comando breve (git) mostrandone l'output attenuato e rientrato.
function Invoke-Quiet([string]$what, [scriptblock]$block) {
    $out = & $block 2>&1
    $code = $LASTEXITCODE
    foreach ($l in $out) { if ("$l".Trim()) { Detail "$l" } }
    if ($code -ne 0) { throw "$what fallito (exit code $code)" }
}

# Output di git da mostrare (log, status), letto come UTF-8: Windows PowerShell
# 5.1 lo decodificherebbe con la codepage OEM della console e l'emoji del
# messaggio di commit apparirebbe come "≡ƒÜÇ" (solo a video: il commit è
# corretto).
function Get-GitOutput([string[]]$gitArgs) {
    $prev = [Console]::OutputEncoding
    try {
        [Console]::OutputEncoding = $utf8
        return (& git @gitArgs)
    } finally {
        [Console]::OutputEncoding = $prev
    }
}

function Show-Asset([string]$rel) {
    $path = Join-Path $root $rel
    if (Test-Path -LiteralPath $path) {
        $item = Get-Item -LiteralPath $path
        Ok ('{0}  ({1:N1} MB)' -f $rel, ($item.Length / 1MB))
    } else {
        Warn "$rel  MANCANTE"
    }
}

# Tutti i file da allegare alla release devono esserci prima del commit: una
# release senza exe o APK non si aggiorna.
function Assert-Assets {
    $missing = @($allAssets | Where-Object { -not (Test-Path -LiteralPath (Join-Path $root $_)) })
    if ($missing.Count -gt 0) { throw ("File da pubblicare mancanti: {0}" -f ($missing -join ', ')) }
}

# Credenziali del client OAuth desktop in internal\google\credentials_local.go
# (ignorato da git): senza, l'exe pubblicato avrebbe l'account Google «Non
# configurato» per tutti. Le variabili RENAMEMUSIC_GOOGLE_CLIENT_ID/_SECRET non
# contano: valgono solo sulla macchina dove sono impostate, non nell'exe.
# Restituisce il client ID, per cercarlo poi nell'exe compilato.
function Get-GoogleClientID {
    if (-not (Test-Path -LiteralPath $credentialsFile)) {
        throw "Manca $credentialsFileRel (ignorato da git): copialo dalla macchina che lo ha o crealo da credentials_local.go.example con le credenziali del client «App desktop». Le variabili d'ambiente non bastano per una release."
    }
    $text = [System.IO.File]::ReadAllText($credentialsFile)
    $id = [regex]::Match($text, '(?m)^\s*clientID\s*=\s*"([^"]*)"').Groups[1].Value
    $secret = [regex]::Match($text, '(?m)^\s*clientSecret\s*=\s*"([^"]*)"').Groups[1].Value
    if ($id -notmatch '^\d+-[a-z0-9]+\.apps\.googleusercontent\.com$') {
        throw "clientID mancante o non valido in $credentialsFileRel (è ancora quello del modello?)."
    }
    if (-not $secret -or $secret -match 'x{8}') {
        throw "clientSecret mancante o non valido in $credentialsFileRel (è ancora quello del modello?)."
    }
    return $id
}

# L'exe deve contenere il client ID (le stringhe Go restano in chiaro nel
# binario): con -NoBuild potrebbe essere stato compilato prima del file.
function Assert-ExeCredentials {
    $bytes = [System.IO.File]::ReadAllBytes((Join-Path $root $desktopAsset))
    $text = [System.Text.Encoding]::GetEncoding(28591).GetString($bytes)
    if ($text.IndexOf($googleClientID, [System.StringComparison]::Ordinal) -lt 0) {
        throw "$desktopAsset non contiene le credenziali di Google di $credentialsFileRel`: ricompilalo (senza -NoBuild)."
    }
    Ok "$desktopAsset contiene le credenziali di Google"
}

# Gli APK devono avere la firma delle versioni pubblicate, altrimenti chi ha
# l'app non potrebbe aggiornarla (con -NoBuild potrebbero venire da un altro
# keystore; build-apk.ps1 controlla già il keystore prima di compilare).
function Assert-ApkSignatures {
    foreach ($a in $apkAssets) {
        Assert-SigningSha1 (Get-ApkSha1 (Join-Path $root $a)) $a
    }
    Ok "APK firmati con il certificato delle versioni pubblicate"
}

# Sposta da parte exe e APK attuali prima delle build: se la release non va in
# porto Restore-Assets li rimette al loro posto, così in build\bin non restano
# file compilati con una versione mai pubblicata (che crederebbero di essere
# più nuovi dell'ultima release e non vedrebbero gli aggiornamenti).
function Backup-Assets {
    if (Test-Path -LiteralPath $backupDir) { Remove-Item -LiteralPath $backupDir -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $backupDir | Out-Null
    foreach ($a in $allAssets) {
        $path = Join-Path $root $a
        if (Test-Path -LiteralPath $path) { Move-Item -LiteralPath $path -Destination $backupDir }
    }
    $script:assetsBackedUp = $true
}

function Restore-Assets {
    foreach ($a in $allAssets) {
        $path = Join-Path $root $a
        $saved = Join-Path $backupDir (Split-Path $a -Leaf)
        if (Test-Path -LiteralPath $saved) {
            Move-Item -LiteralPath $saved -Destination $path -Force
        } elseif (Test-Path -LiteralPath $path) {
            # Non c'era prima delle build: è solo della versione non pubblicata.
            Remove-Item -LiteralPath $path -Force
        }
    }
    Remove-Item -LiteralPath $backupDir -Recurse -Force -ErrorAction SilentlyContinue
    $script:assetsBackedUp = $false
}

# Le note di rilascio si scrivono solo qui: un file temporaneo vuoto aperto nel
# Blocco note, letto quando si preme Invio ed eliminato subito dopo (anche se
# si annulla con Ctrl+C). Il testo resta solo in memoria.
function Read-ReleaseNotes {
    $file = Join-Path ([System.IO.Path]::GetTempPath()) ("RenameMusic-note-{0}-{1}.txt" -f $Version, [guid]::NewGuid().ToString('N').Substring(0, 8))
    try {
        [System.IO.File]::WriteAllText($file, '', $utf8)
        Start-Process notepad.exe -ArgumentList "`"$file`""
        Write-Host '  ' -NoNewline
        Badge '?' Black Magenta
        Write-Host ' Scrivi le note di rilascio nel Blocco note (compaiono nel popup di aggiornamento),' -ForegroundColor White
        Write-Host '      salva e premi Invio qui per continuare ' -NoNewline -ForegroundColor White
        Read-Host | Out-Null
        # Senza encoding esplicito ReadAllText riconosce il BOM e altrimenti
        # legge in UTF-8, come salva il Blocco note.
        return [System.IO.File]::ReadAllText($file).Trim()
    } finally {
        Remove-Item -LiteralPath $file -Force -ErrorAction SilentlyContinue
    }
}

# ---- Stato, per spiegare come tornare indietro se qualcosa va storto ---------------

$versionWritten = $false
$indexWasClean = $true
$built = $false
$staged = $false
$committed = $false
$tagged = $false
$pushed = $false
$assetsBackedUp = $false
$ghReady = $false
$tag = ''
$retag = $false
$oldLocalTag = ''

# Annulla da solo quello che lo script ha fatto prima del commit (versione in
# update.go, staging, exe e APK), così un annullamento o un errore riportano il
# repository com'era. Dopo il commit non tocca nulla: ci pensa Show-Recovery.
function Undo-Local {
    if ($committed -or $pushed) { return }
    if ($assetsBackedUp) {
        Restore-Assets
        Ok 'Exe e APK in build\bin ripristinati a quelli di prima delle build'
    }
    if ($staged -and $indexWasClean) {
        git reset -q
        $script:staged = $false
        Ok 'Staging annullato'
    }
    if ($versionWritten) {
        [System.IO.File]::WriteAllText($versionFile, $source, $utf8)
        $script:versionWritten = $false
        Ok "Versione in $versionFileRel ripristinata a $current"
    }
}

# Spiega cosa resta da sistemare a mano (tipicamente: push fallito).
function Show-Recovery {
    $hints = @()
    if ($tagged -and $retag) {
        $hints += "Il tag $tag è stato ricreato solo in locale. Riprova il push:  git push --atomic origin HEAD +refs/tags/${tag}:refs/tags/${tag}"
        if ($oldLocalTag) {
            $hints += "oppure rimetti quello di prima:  git tag -f $tag $oldLocalTag"
        } else {
            $hints += "oppure eliminalo:  git tag -d $tag"
        }
    } elseif ($tagged) {
        $hints += "Il tag $tag esiste solo in locale. Riprova il push:  git push --atomic origin HEAD $tag"
        $hints += "oppure eliminalo:  git tag -d $tag"
    }
    if ($committed) {
        $hints += 'Il commit di release esiste solo in locale. Per annullarlo:  git reset --soft HEAD~1'
        if ($assetsBackedUp) {
            $hints += "In quel caso exe e APK di prima delle build sono in $backupDir"
        }
    } elseif ($staged) {
        $hints += 'Le modifiche sono in staging insieme a quelle che c''erano già. Per toglierle:  git reset'
    }
    Write-Host ''
    if ($hints.Count -eq 0) {
        $note = if ($built) { ' (a parte i file intermedi rigenerati dalle build)' } else { '' }
        Write-Host "  Il repository è com'era prima della release$note." -ForegroundColor Gray
        return
    }
    Write-Host '  Da sistemare a mano' -ForegroundColor White
    foreach ($h in $hints) { Write-Host "    - $h" -ForegroundColor Yellow }
}

# ---- Argomento obbligatorio -----------------------------------------------------

$current = ''
try {
    $source = [System.IO.File]::ReadAllText($versionFile, $utf8)
    $m = [regex]::Match($source, '(?m)^const Version = "(\d+\.\d+\.\d+)"')
    if ($m.Success) { $current = $m.Groups[1].Value }
} catch {
    $source = ''
}
if (-not $current) {
    Banner 'ERRORE' White DarkRed Red "Versione non trovata in $versionFileRel (const Version = `"X.Y.Z`")."
    exit 1
}

$requested = @()
if ($Bump) { $requested += $Bump }
if ($Patch) { $requested += 'patch' }
if ($Minor) { $requested += 'minor' }
if ($Major) { $requested += 'major' }
if ($Rest) { $requested += $Rest }
$requested = @($requested | Where-Object { "$_".Trim() } | ForEach-Object { "$_".Trim() })
$validKinds = @('patch', 'minor', 'major')
$unknownOptions = @($requested | Where-Object { $_.StartsWith('-') -and $_.TrimStart('-').ToLower() -notin $validKinds })
if ($unknownOptions.Count -gt 0) {
    Fail-Usage ("Opzione sconosciuta: {0}" -f ($unknownOptions -join ', ')) $current
}
$kinds = @($requested | ForEach-Object { $_.TrimStart('-').ToLower() } | Select-Object -Unique)

if ($kinds.Count -eq 0) {
    Fail-Usage 'Manca il tipo di release: indica patch, minor o major.' $current
}
if ($kinds.Count -gt 1) {
    Fail-Usage ("Indica un solo tipo di release (ricevuti: {0})." -f ($requested -join ', ')) $current
}
$kind = $kinds[0]
if ($kind -notin $validKinds) {
    Fail-Usage "Tipo di release non valido: '$($requested[0])'. Usa patch, minor o major." $current
}

$Version = Get-NextVersion $current $kind
$tag = "v$Version"
$message = "chore(release): 🚀 release $Version"

# ---- Release --------------------------------------------------------------------

Push-Location $root
try {
    $branch = (git symbolic-ref --short -q HEAD)
    if ($LASTEXITCODE -ne 0 -or -not $branch) { $branch = '' }

    Write-Host ''
    Line '═' Cyan
    Write-Host '  RenameMusic' -NoNewline -ForegroundColor White
    Write-Host '  release ' -NoNewline -ForegroundColor DarkGray
    Write-Host $kind -ForegroundColor Yellow
    Write-Host '  ' -NoNewline
    Write-Host $current -NoNewline -ForegroundColor Gray
    Write-Host '  ->  ' -NoNewline -ForegroundColor DarkGray
    Write-Host $Version -NoNewline -ForegroundColor Green
    Write-Host "     tag $tag" -NoNewline -ForegroundColor DarkGray
    if ($branch) { Write-Host " · branch $branch" -ForegroundColor DarkGray } else { Write-Host '' }
    Line '═' Cyan

    # ---- 1. Controlli -------------------------------------------------------------

    Step 1 'Controlli'
    if (-not $branch) { throw 'HEAD non è su un branch (detached): passa prima a un branch.' }
    Ok "Branch $branch"

    $googleClientID = Get-GoogleClientID
    Ok "Credenziali di Google in $credentialsFileRel"

    $signing = Get-SigningConfig (Join-Path $root 'frontend\android')
    Ok "Keystore per gli APK: $($signing.storeFile)"

    $oldLocalTag = git rev-parse -q --verify "refs/tags/$tag"
    if ($LASTEXITCODE -ne 0) { $oldLocalTag = '' }
    Detail 'Controllo dei tag su origin...'
    $remoteTag = git ls-remote --tags origin "refs/tags/$tag"
    if ($LASTEXITCODE -ne 0) { throw 'Impossibile contattare origin: sei connesso a Internet?' }
    if ($oldLocalTag -or $remoteTag) {
        # Tipicamente una release precedente annullata a metà. Il tag esistente
        # non si tocca qui: lo sostituiscono git tag -f e il push (atomico),
        # quindi un annullamento o un errore prima del push lo lasciano com'è.
        $where = @()
        if ($oldLocalTag) { $where += 'in locale' }
        if ($remoteTag) { $where += 'su origin' }
        $where = $where -join ' e '
        Warn "Il tag $tag esiste già $where"
        if ($Yes) { throw "Il tag $tag esiste già $where (con -Yes non viene ricreato: rilancia senza -Yes o eliminalo a mano)." }
        Write-Host '  ' -NoNewline
        Badge '?' Black Magenta
        Write-Host " Eliminarlo e ricrearlo sul nuovo commit di release? " -NoNewline -ForegroundColor White
        Write-Host '[s = ricrea / N = annulla] ' -NoNewline -ForegroundColor Yellow
        $answer = Read-Host
        if ($answer -notmatch '^(s|si|sì|y|yes)$') {
            Banner 'ANNULLATO' Black Yellow Yellow "Nessuna modifica: il tag $tag è rimasto com'era."
            Write-Host ''
            exit 1
        }
        $retag = $true
        Ok "Il tag $tag verrà ricreato (quello attuale resta fino al push)"
    } else {
        Ok "Tag $tag libero (in locale e su origin)"
    }

    git diff --cached --quiet
    $indexWasClean = $LASTEXITCODE -eq 0
    $dirty = @(git status --porcelain)
    if ($dirty.Count -gt 0) {
        Warn "$($dirty.Count) modifiche non committate: finiranno nel commit di release"
        foreach ($d in $dirty) { Detail $d }
    } else {
        Ok 'Working tree pulito'
    }

    # wails dev ricompila l'app di sviluppo a ogni modifica di update.go: resta
    # con la versione nuova anche se la release poi si annulla.
    if (Get-Process 'RenameMusic-dev' -ErrorAction SilentlyContinue) {
        Warn 'wails dev è in esecuzione: l''app di sviluppo prenderà la versione nuova; se la release si annulla riavvialo'
    }

    # Release su GitHub automatica solo con gh installata e autenticata; le note
    # si scrivono subito, per non accorgersene dopo build e push.
    if (Get-Command gh -ErrorAction SilentlyContinue) {
        gh auth status --hostname github.com *> $null
        $ghReady = $LASTEXITCODE -eq 0
        if (-not $ghReady) { Warn 'GitHub CLI non autenticata (gh auth login): la release su GitHub sarà da creare a mano' }
    } else {
        Warn 'GitHub CLI non trovata (winget install GitHub.cli): la release su GitHub sarà da creare a mano'
    }
    if ($ghReady) {
        $notesText = Read-ReleaseNotes
        if (-not $notesText) {
            throw 'Note di rilascio vuote: scrivile nel Blocco note e salva prima di premere Invio.'
        }
        Ok ("Release su GitHub automatica (gh), note scritte ({0} righe)" -f @($notesText -split "`r?`n").Count)
    }

    # ---- 2. Versione --------------------------------------------------------------

    Step 2 'Versione'
    $updated = $source.Substring(0, $m.Groups[1].Index) + $Version +
        $source.Substring($m.Groups[1].Index + $m.Groups[1].Length)
    [System.IO.File]::WriteAllText($versionFile, $updated, $utf8)
    $versionWritten = $true
    Ok "$versionFileRel  $current -> $Version"

    # ---- 3-4. Build ---------------------------------------------------------------

    if ($NoBuild) {
        Step 3 'Build desktop'
        Warn 'Saltata (-NoBuild): l''exe in build\bin deve essere già compilato con la versione nuova'
        Show-Asset $desktopAsset
        Step 4 'Build Android'
        Warn 'Saltata (-NoBuild): gli APK in build\bin devono essere già compilati con la versione nuova'
        foreach ($a in $apkAssets) { Show-Asset $a }
    } else {
        $env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"
        $built = $true
        Backup-Assets

        Step 3 'Build desktop (wails build)'
        Invoke-Checked 'wails build' { wails build }
        Show-Asset $desktopAsset

        Step 4 'Build Android (mobile\build-apk.ps1)'
        $apkArgs = @('-ExecutionPolicy', 'Bypass', '-File', (Join-Path $root 'mobile\build-apk.ps1'))
        if ($JavaHome) { $apkArgs += @('-JavaHome', $JavaHome) }
        Invoke-Checked 'build-apk.ps1' { powershell @apkArgs }
        foreach ($a in $apkAssets) { Show-Asset $a }
    }
    Assert-Assets
    Assert-ExeCredentials
    Assert-ApkSignatures

    # ---- 5. Commit, tag, push -----------------------------------------------------

    Step 5 'Commit, tag e push'
    # core.safecrlf=false: niente avvisi "LF will be replaced by CRLF".
    Invoke-Quiet 'git add' { git -c core.safecrlf=false add -A }
    $staged = $true
    git diff --cached --quiet
    $hasChanges = $LASTEXITCODE -ne 0
    if ($hasChanges) {
        Write-Host '  File nel commit' -ForegroundColor White
        foreach ($f in (Get-GitOutput @('status', '--short'))) { Detail $f }
        Write-Host '  Messaggio      ' -NoNewline -ForegroundColor White
        Write-Host $message -ForegroundColor Green
    } else {
        Warn "Nessuna modifica da committare: il tag $tag andrà sul commit corrente"
        Detail (Get-GitOutput @('log', '--oneline', '-1'))
    }
    Write-Host '  Tag            ' -NoNewline -ForegroundColor White
    Write-Host $tag -NoNewline -ForegroundColor Green
    if ($retag) { Write-Host ' (ricreato: sostituisce quello esistente)' -ForegroundColor Yellow } else { Write-Host '' }
    Write-Host '  Push su        ' -NoNewline -ForegroundColor White
    Write-Host "origin/$branch" -ForegroundColor Green
    Write-Host '  Release GitHub ' -NoNewline -ForegroundColor White
    if ($ghReady) { Write-Host "RenameMusic $Version, note scritte al passo 1, 3 file allegati" -ForegroundColor Green }
    else { Write-Host 'da creare a mano nel browser' -ForegroundColor Yellow }

    if (-not $Yes) {
        Write-Host ''
        Write-Host '  ' -NoNewline
        Badge '?' Black Magenta
        $what = if ($ghReady) { 'commit, tag, push e release su GitHub' } else { 'commit, tag e push' }
        Write-Host " Procedere con ${what}? " -NoNewline -ForegroundColor White
        Write-Host '[s/N] ' -NoNewline -ForegroundColor Yellow
        $answer = Read-Host
        if ($answer -notmatch '^(s|si|sì|y|yes)$') {
            Banner 'ANNULLATO' Black Yellow Yellow 'Nessun commit, tag o push è stato fatto.'
            Undo-Local
            Show-Recovery
            Write-Host ''
            exit 1
        }
        Write-Host ''
    }

    if ($hasChanges) {
        # Il messaggio passa da un file UTF-8: così l'emoji arriva intatta a git
        # anche da Windows PowerShell 5.1.
        $msgFile = [System.IO.Path]::GetTempFileName()
        try {
            [System.IO.File]::WriteAllText($msgFile, $message, $utf8)
            Invoke-Quiet 'git commit' { git commit -q -F $msgFile }
        } finally {
            Remove-Item $msgFile -ErrorAction SilentlyContinue
        }
        $committed = $true
        Ok ("Commit {0}" -f (Get-GitOutput @('log', '--oneline', '-1')))
    }

    if ($retag) {
        Invoke-Quiet 'git tag' { git tag -a -f $tag -m "RenameMusic $Version" }
    } else {
        Invoke-Quiet 'git tag' { git tag -a $tag -m "RenameMusic $Version" }
    }
    $tagged = $true
    Ok "Tag $tag"

    Detail "Push su origin ($branch + $tag)..."
    if ($retag) {
        # Il + forza solo il tag (sostituisce quello su origin), non il branch.
        Invoke-Quiet 'git push' { git push --atomic origin HEAD "+refs/tags/${tag}:refs/tags/${tag}" }
    } else {
        Invoke-Quiet 'git push' { git push --atomic origin HEAD $tag }
    }
    # Da qui la versione è pubblicata: un errore successivo non deve più
    # annullare nulla in locale (versione, exe e APK sono quelli giusti).
    $pushed = $true
    $tagged = $false
    $committed = $false
    if ($assetsBackedUp) {
        Remove-Item -LiteralPath $backupDir -Recurse -Force -ErrorAction SilentlyContinue
        $assetsBackedUp = $false
    }
    Ok "Push di $branch e $tag su origin"

    # ---- 6. Release su GitHub -----------------------------------------------------

    Step 6 'Release su GitHub'
    $assetPaths = @($allAssets | ForEach-Object { Join-Path $root $_ })
    $published = $false
    if ($ghReady -and $retag) {
        # Una release (anche bozza) rimasta sul vecchio tag farebbe fallire
        # gh release create: si elimina, senza toccare il tag appena pushato.
        gh release view $tag --repo $repoSlug *> $null
        if ($LASTEXITCODE -eq 0) {
            Detail "Eliminazione della release $tag rimasta dal vecchio tag..."
            gh release delete $tag --repo $repoSlug --yes
            if ($LASTEXITCODE -eq 0) { Ok "Vecchia release $tag eliminata" }
            else { Warn "Impossibile eliminare la vecchia release ${tag}: eliminala da $repoUrl/releases" }
        }
    }
    if ($ghReady) {
        Detail 'Creazione della release e caricamento di exe e APK...'
        # gh carica i file col loro nome (quelli esatti che cerca l'app) e
        # pubblica la release solo a caricamento finito. --verify-tag: il tag
        # deve già essere su origin (appena fatto il push).
        # Le note passano da un file temporaneo che esiste solo per questa
        # chiamata: come argomento, PowerShell 5.1 rovinerebbe virgolette e a capo.
        $ghNotes = [System.IO.Path]::GetTempFileName()
        try {
            [System.IO.File]::WriteAllText($ghNotes, $notesText, $utf8)
            gh release create $tag @assetPaths --repo $repoSlug --title "RenameMusic $Version" --notes-file $ghNotes --verify-tag
            $ghCode = $LASTEXITCODE
        } finally {
            Remove-Item -LiteralPath $ghNotes -Force -ErrorAction SilentlyContinue
        }
        if ($ghCode -eq 0) {
            $published = $true
            $releaseUrl = "$repoUrl/releases/tag/$tag"
            Ok "Release pubblicata: $releaseUrl"
            Start-Process $releaseUrl
        } else {
            Warn "gh release create fallito (exit code $ghCode)"
            Detail "Controlla $repoUrl/releases: se è rimasta una bozza, eliminala o completala."
            Detail "Oppure creala a mano da $repoUrl/releases/new?tag=$tag allegando:"
            foreach ($p in $assetPaths) { Detail "  $p" }
            Detail 'con queste note (copiale da qui, non sono salvate altrove):'
            Write-Host ''
            foreach ($l in ($notesText -split "`r?`n")) { Write-Host "    $l" -ForegroundColor Cyan }
        }
    } else {
        $releaseUrl = "$repoUrl/releases/new?tag=$tag&title=RenameMusic%20$Version"
        Write-Host '  Nella pagina aperta nel browser:' -ForegroundColor White
        Write-Host '    1. allega questi file, con questi nomi esatti:' -ForegroundColor Gray
        foreach ($p in $assetPaths) { Write-Host "         $p" -ForegroundColor Cyan }
        Write-Host "    2. scrivi le novità nella descrizione (compaiono nel popup di aggiornamento)" -ForegroundColor Gray
        Write-Host '    3. lascia spento "Set as a pre-release" e premi "Publish release"' -ForegroundColor Gray
        Detail $releaseUrl
        Start-Process $releaseUrl
    }

    if ($published) {
        Banner 'FATTO' Black Green Green "RenameMusic ${Version} pubblicata: tag $tag su origin/$branch  ($(Elapsed))"
    } else {
        Banner 'FATTO' Black Green Green "RenameMusic ${Version}: tag $tag su origin/$branch  ($(Elapsed))"
        Write-Host '  Manca solo pubblicare la release su GitHub con i 3 file allegati.' -ForegroundColor Gray
    }
    Write-Host ''
} catch {
    Banner 'ERRORE' White DarkRed Red "Release interrotta: $($_.Exception.Message)"
    Undo-Local
    Show-Recovery
    Write-Host ''
    exit 1
} finally {
    Pop-Location
}
