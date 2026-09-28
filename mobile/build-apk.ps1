<#
.SYNOPSIS
    Compila l'APK Android di RenameMusic.

.DESCRIPTION
    1. core Go (internal/core + mobile) -> libreria .aar con gomobile bind
    2. frontend React (stessa UI del desktop) -> dist/ con Vite
    3. copia di dist/ e plugin nel progetto Android (npx cap sync)
    4. APK con Gradle (uno per architettura: arm64-v8a per i telefoni,
       x86_64 per l'emulatore)

    Gli APK finiscono in build\bin\ (RenameMusic-arm64-v8a.apk, ...). Cache e
    file temporanei (Go, Gradle, npm) stanno in .android-build\, nella cartella
    del progetto: non occupano spazio sul disco di sistema.

    Requisiti: Go, Node/npm, Android SDK con NDK, JDK 21 e gomobile:
        go install golang.org/x/mobile/cmd/gomobile@<versione in go.mod>
        go install golang.org/x/mobile/cmd/gobind@<versione in go.mod>

.PARAMETER JavaHome
    JDK 21 da usare per Gradle. Default: il JDK del comando `java` nel PATH
    (JAVA_HOME viene ignorata se punta a un JDK più vecchio del 21).

.PARAMETER SkipGo
    Non ricompila il core Go (riusa l'.aar già presente in app\libs).

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File mobile\build-apk.ps1
#>
param(
    [string]$JavaHome = '',
    [switch]$SkipGo
)

# 'Continue' e non 'Stop': in Windows PowerShell 5.1 con 'Stop' qualunque
# scrittura su stderr di un comando nativo (java, npm, Gradle la usano anche per
# messaggi normali) interromperebbe lo script. Gli errori veri si intercettano
# con l'exit code (Invoke-Checked) e con throw espliciti.
$ErrorActionPreference = 'Continue'

$root = Split-Path -Parent $PSScriptRoot
$frontend = Join-Path $root 'frontend'
$android = Join-Path $frontend 'android'
$work = Join-Path $root '.android-build'
$libs = Join-Path $android 'app\libs'
$out = Join-Path $root 'build\bin'

function Step($text) { Write-Host "`n==> $text" -ForegroundColor Cyan }

function Invoke-Checked([string]$what, [scriptblock]$block) {
    & $block
    if ($LASTEXITCODE -ne 0) { throw "$what fallito (exit code $LASTEXITCODE)" }
}

# Versione major di un JDK (es. 21) letta dal file "release" della sua cartella.
function Get-JdkMajor([string]$jdkDir) {
    $jdkDir = $jdkDir.Trim()
    if (-not $jdkDir) { return 0 }
    $release = Join-Path $jdkDir 'release'
    if (-not (Test-Path -LiteralPath $release -ErrorAction SilentlyContinue)) { return 0 }
    $line = Select-String -Path $release -Pattern '^JAVA_VERSION="(\d+)' | Select-Object -First 1
    if ($null -eq $line) { return 0 }
    return [int]$line.Matches[0].Groups[1].Value
}

# ---- Ambiente -------------------------------------------------------------

foreach ($d in @($work, "$work\tmp", "$work\gocache", "$work\gradle", "$work\npm-cache", $libs, $out)) {
    New-Item -ItemType Directory -Force -Path $d | Out-Null
}
$env:TMP = "$work\tmp"
$env:TEMP = "$work\tmp"
$env:GOTMPDIR = "$work\tmp"
$env:GOCACHE = "$work\gocache"
$env:GRADLE_USER_HOME = "$work\gradle"
$env:npm_config_cache = "$work\npm-cache"

if (-not $env:ANDROID_HOME) { $env:ANDROID_HOME = Join-Path $env:LOCALAPPDATA 'Android\Sdk' }
if (-not (Test-Path $env:ANDROID_HOME)) { throw "Android SDK non trovato: imposta ANDROID_HOME." }
if (-not $env:ANDROID_NDK_HOME) {
    $ndk = Get-ChildItem (Join-Path $env:ANDROID_HOME 'ndk') -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending | Select-Object -First 1
    if ($null -eq $ndk) { throw "Android NDK non trovato: installalo dall'SDK Manager o imposta ANDROID_NDK_HOME." }
    $env:ANDROID_NDK_HOME = $ndk.FullName
}

if (-not $JavaHome) {
    $envJava = "$env:JAVA_HOME".Trim()
    if ($envJava -and (Get-JdkMajor $envJava) -ge 21) {
        $JavaHome = $envJava
    } else {
        $props = cmd /c 'java -XshowSettings:properties -version 2>&1' | Out-String
        $m = [regex]::Match($props, 'java\.home = (.+)')
        if ($m.Success) { $JavaHome = $m.Groups[1].Value.Trim() }
    }
}
if (-not $JavaHome -or (Get-JdkMajor $JavaHome) -lt 21) {
    throw "Serve un JDK 21 o successivo per Gradle: passa -JavaHome <cartella del JDK>."
}
$env:JAVA_HOME = $JavaHome
$env:PATH = "$JavaHome\bin;$env:USERPROFILE\go\bin;$env:PATH"

# Versione dell'app: unica fonte è update.Version in internal/update/update.go.
# versionCode = major*10000 + minor*100 + patch, così cresce a ogni release
# (Android rifiuta un aggiornamento con versionCode minore di quello installato).
$versionFile = Join-Path $root 'internal\update\update.go'
$vm = Select-String -Path $versionFile -Pattern '^const Version = "(\d+)\.(\d+)\.(\d+)"' | Select-Object -First 1
if ($null -eq $vm) { throw "Versione non trovata in $versionFile (const Version = `"X.Y.Z`")." }
$g = $vm.Matches[0].Groups
$appVersionName = "$($g[1].Value).$($g[2].Value).$($g[3].Value)"
$appVersionCode = [int]$g[1].Value * 10000 + [int]$g[2].Value * 100 + [int]$g[3].Value

Write-Host "Versione:    $appVersionName (versionCode $appVersionCode)"
Write-Host "Android SDK: $env:ANDROID_HOME"
Write-Host "Android NDK: $env:ANDROID_NDK_HOME"
Write-Host "JDK:         $JavaHome"

# ---- 1. Core Go -> .aar ---------------------------------------------------

if (-not $SkipGo) {
    Step 'Core Go -> renamemusic.aar (gomobile bind)'
    if (-not (Get-Command gomobile -ErrorAction SilentlyContinue)) {
        throw "gomobile non trovato: vedi i requisiti in testa allo script."
    }
    Push-Location $root
    try {
        Invoke-Checked 'gomobile bind' {
            gomobile bind -target='android/arm64,android/amd64' -androidapi 24 `
                -javapkg='com.renamemusic.gobind' -ldflags='-s -w' `
                -o (Join-Path $libs 'renamemusic.aar') ./mobile
        }
    } finally {
        Pop-Location
    }
}

# ---- 2-3. Frontend -> progetto Android ------------------------------------

Push-Location $frontend
try {
    Step 'Frontend (tsc + Vite)'
    Invoke-Checked 'npm run build' { npm run build }
    Step 'Sincronizzazione con il progetto Android (cap sync)'
    Invoke-Checked 'npx cap sync' { npx cap sync android }
} finally {
    Pop-Location
}

# ---- 4. APK ----------------------------------------------------------------

Step 'APK (Gradle assembleDebug)'
Push-Location $android
try {
    Invoke-Checked 'Gradle' {
        .\gradlew.bat assembleDebug --console=plain `
            "-PappVersionName=$appVersionName" "-PappVersionCode=$appVersionCode"
    }
} finally {
    Pop-Location
}

$apkDir = Join-Path $android 'app\build\outputs\apk\debug'
$apks = Get-ChildItem $apkDir -Filter '*.apk'
foreach ($apk in $apks) {
    # app-arm64-v8a-debug.apk -> RenameMusic-arm64-v8a.apk
    $abi = $apk.BaseName -replace '^app-', '' -replace '-debug$', ''
    $dest = Join-Path $out "RenameMusic-$abi.apk"
    Copy-Item $apk.FullName $dest -Force
    Write-Host ("APK: {0} ({1:N1} MB)" -f $dest, ($apk.Length / 1MB)) -ForegroundColor Green
}
Write-Host "`nPer il telefono usa RenameMusic-arm64-v8a.apk."
