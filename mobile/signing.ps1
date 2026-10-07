# Firma degli APK: funzioni comuni a build-apk.ps1 e release.ps1 (dot-source).
#
# Il keystore sta fuori dal repo e lo indica frontend\android\keystore.properties
# (ignorato da git, modello in keystore.properties.example), letto anche da
# app\build.gradle. Deve essere sempre lo stesso: Android accetta un
# aggiornamento solo con la firma dell'app installata, e il client OAuth Android
# di Google riconosce l'app dallo SHA-1 del certificato. Perciò il certificato
# atteso è fissato qui: con un keystore diverso build e release si fermano.

$SigningCertSha1 = '50:6C:9E:18:9D:7E:E2:CC:4D:20:F6:83:FB:63:BF:3D:79:05:07:9B'

# Legge keystore.properties (lo stesso file di Gradle) e controlla che il
# keystore esista. Restituisce le proprietà, con storeFile come percorso assoluto.
function Get-SigningConfig([string]$androidDir) {
    $propsFile = Join-Path $androidDir 'keystore.properties'
    if (-not (Test-Path -LiteralPath $propsFile)) {
        throw "Manca frontend\android\keystore.properties (ignorato da git): crealo da keystore.properties.example indicando il keystore delle versioni pubblicate."
    }
    $props = @{}
    foreach ($line in [System.IO.File]::ReadAllLines($propsFile)) {
        $m = [regex]::Match($line, '^\s*([^#!\s=:][^=:]*?)\s*[=:]\s*(.*?)\s*$')
        if ($m.Success) { $props[$m.Groups[1].Value] = $m.Groups[2].Value }
    }
    foreach ($key in @('storeFile', 'storePassword', 'keyAlias', 'keyPassword')) {
        if (-not $props[$key]) { throw "Manca $key in frontend\android\keystore.properties." }
    }
    # Nei .properties \ è un carattere di escape: Gradle leggerebbe un altro percorso.
    if ($props.storeFile.Contains('\')) {
        throw "In frontend\android\keystore.properties storeFile deve usare / al posto di \."
    }
    $store = $props.storeFile
    if (-not [System.IO.Path]::IsPathRooted($store)) { $store = Join-Path $androidDir $store }
    if (-not (Test-Path -LiteralPath $store -PathType Leaf)) {
        throw "Keystore non trovato: $store (storeFile in frontend\android\keystore.properties)."
    }
    $props.storeFile = [System.IO.Path]::GetFullPath($store)
    return $props
}

# SHA-1 (XX:XX:...) del certificato della chiave indicata in keystore.properties.
function Get-KeystoreSha1([string]$javaHome, [hashtable]$cfg) {
    $keytool = Join-Path $javaHome 'bin\keytool.exe'
    $out = & $keytool '-J-Duser.language=en' -list -v -keystore $cfg.storeFile `
        -storepass $cfg.storePassword -alias $cfg.keyAlias 2>&1 | ForEach-Object { "$_" } | Out-String
    $m = [regex]::Match($out, 'SHA1:\s*([0-9A-F]{2}(?::[0-9A-F]{2}){19})')
    if ($LASTEXITCODE -ne 0 -or -not $m.Success) {
        throw "keytool non riesce a leggere la chiave $($cfg.keyAlias) di $($cfg.storeFile) (password o alias sbagliati?):`n$(($out.Trim() -split "`r?`n")[0])"
    }
    return $m.Groups[1].Value
}

# SHA-1 (XX:XX:...) del certificato con cui è firmato un APK (apksigner
# dell'Android SDK: gli APK con minSdk 24 hanno solo le firme v2/v3, che keytool
# non legge).
function Get-ApkSha1([string]$apk) {
    $sdk = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } else { Join-Path $env:LOCALAPPDATA 'Android\Sdk' }
    $buildTools = Get-ChildItem (Join-Path $sdk 'build-tools') -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending | Select-Object -First 1
    if ($null -eq $buildTools) { throw "Android SDK Build-Tools non trovati in $sdk (servono per apksigner)." }
    $out = & (Join-Path $buildTools.FullName 'apksigner.bat') verify --print-certs $apk 2>&1 | ForEach-Object { "$_" } | Out-String
    $m = [regex]::Match($out, 'certificate SHA-1 digest:\s*([0-9a-fA-F]{40})')
    if ($LASTEXITCODE -ne 0 -or -not $m.Success) {
        throw "Firma di $apk non valida o illeggibile:`n$($out.Trim())"
    }
    return (($m.Groups[1].Value.ToUpper() -split '(..)' | Where-Object { $_ }) -join ':')
}

# Si ferma se lo SHA-1 non è quello delle versioni pubblicate.
function Assert-SigningSha1([string]$sha1, [string]$what) {
    if ($sha1 -ne $SigningCertSha1) {
        throw "$what ha un certificato diverso da quello delle versioni pubblicate (SHA-1 $sha1, atteso $SigningCertSha1): gli aggiornamenti verrebbero rifiutati e l'accesso a Google non funzionerebbe."
    }
}
