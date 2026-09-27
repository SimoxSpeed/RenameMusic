<#
.SYNOPSIS
    Installa l'APK di RenameMusic sul telefono (o sull'emulatore) con adb.

.DESCRIPTION
    Sceglie l'APK adatto all'architettura del dispositivo collegato
    (arm64-v8a per i telefoni, x86_64 per l'emulatore) fra quelli prodotti da
    build-apk.ps1 in build\bin\, e lo installa aggiornando l'app esistente
    (i dati e le impostazioni dell'app restano).

    Sul telefono vanno attivati "Opzioni sviluppatore" e "Debug USB" (o
    "Debug wireless"), e al primo collegamento va accettata l'autorizzazione
    del PC.

.PARAMETER Build
    Compila prima l'APK (esegue build-apk.ps1).

.PARAMETER Serial
    Dispositivo su cui installare (come elencato da `adb devices`). Serve solo
    se ne sono collegati più di uno (es. telefono ed emulatore).

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File mobile\install-apk.ps1 -Build
#>
param(
    [switch]$Build,
    [string]$Serial = ''
)

# Vedi build-apk.ps1: con 'Stop' lo stderr di adb interromperebbe lo script.
$ErrorActionPreference = 'Continue'

$root = Split-Path -Parent $PSScriptRoot
$out = Join-Path $root 'build\bin'

if ($Build) {
    & (Join-Path $PSScriptRoot 'build-apk.ps1')
    if (-not $?) { throw 'Compilazione dell''APK fallita.' }
}

$sdk = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } else { Join-Path $env:LOCALAPPDATA 'Android\Sdk' }
$adb = Join-Path $sdk 'platform-tools\adb.exe'
if (-not (Test-Path $adb)) { throw "adb non trovato in $adb (installa 'Android SDK Platform-Tools' o imposta ANDROID_HOME)." }

# Dispositivi pronti ("device"): esclude quelli non autorizzati o offline.
$lines = & $adb devices | Select-Object -Skip 1 | Where-Object { $_ -match '\S' }
$ready = @($lines | Where-Object { $_ -match '\tdevice$' } | ForEach-Object { ($_ -split '\t')[0] })
$other = @($lines | Where-Object { $_ -notmatch '\tdevice$' })

if (-not $Serial) {
    if ($ready.Count -eq 0) {
        if ($other.Count -gt 0) {
            throw "Dispositivo collegato ma non pronto:`n$($other -join "`n")`nSblocca il telefono e accetta l'autorizzazione al debug USB."
        }
        throw 'Nessun dispositivo collegato: collega il telefono con il debug USB attivo (o avvia l''emulatore).'
    }
    if ($ready.Count -gt 1) {
        throw "Più dispositivi collegati ($($ready -join ', ')): scegli con -Serial <nome>."
    }
    $Serial = $ready[0]
}

$abis = (& $adb -s $Serial shell getprop ro.product.cpu.abilist | Out-String).Trim() -split ','
$apk = $null
foreach ($abi in $abis) {
    $candidate = Join-Path $out "RenameMusic-$($abi.Trim()).apk"
    if (Test-Path $candidate) { $apk = $candidate; break }
}
if (-not $apk) {
    throw "Nessun APK in $out per le architetture del dispositivo ($($abis -join ', ')): compila con -Build."
}

$item = Get-Item $apk
Write-Host ("Installo {0} ({1:N1} MB, del {2:g}) su {3}..." -f $item.Name, ($item.Length / 1MB), $item.LastWriteTime, $Serial) -ForegroundColor Cyan
& $adb -s $Serial install -r $apk
if ($LASTEXITCODE -ne 0) { throw "Installazione fallita (exit code $LASTEXITCODE)." }
Write-Host 'Installata.' -ForegroundColor Green
