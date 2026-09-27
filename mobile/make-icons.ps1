<#
.SYNOPSIS
    Genera le icone Android dell'app a partire da build\appicon.png (la stessa
    icona dell'app desktop).

.DESCRIPTION
    - mipmap-*/ic_launcher.png e ic_launcher_round.png: icone classiche (48dp)
    - mipmap-*/ic_launcher_foreground.png: primo piano dell'icona adattiva
      (108dp), con il cerchio dell'icona ridotto all'area visibile della
      maschera del launcher; lo sfondo è il colore del cerchio
      (values/ic_launcher_background.xml), così anche le maschere non circolari
      restano piene.
    Va rieseguito solo se cambia build\appicon.png.
#>
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$root = Split-Path -Parent $PSScriptRoot
$res = Join-Path $root 'frontend\android\app\src\main\res'
$src = [System.Drawing.Bitmap]::FromFile((Join-Path $root 'build\appicon.png'))

# Parte del riquadro da 108dp occupata dal cerchio: poco più dell'area visibile
# (72dp), così una maschera circolare taglia via il bordo sfumato del cerchio.
$foregroundScale = 0.70

function Save-Icon([string]$path, [int]$size, [double]$scale) {
    $bmp = New-Object System.Drawing.Bitmap $size, $size
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.Clear([System.Drawing.Color]::Transparent)
    $d = [int][Math]::Round($size * $scale)
    $o = [int][Math]::Round(($size - $d) / 2)
    $g.DrawImage($src, $o, $o, $d, $d)
    $g.Dispose()
    $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
}

$densities = @{ 'mdpi' = 1.0; 'hdpi' = 1.5; 'xhdpi' = 2.0; 'xxhdpi' = 3.0; 'xxxhdpi' = 4.0 }
foreach ($name in $densities.Keys) {
    $k = $densities[$name]
    $dir = Join-Path $res "mipmap-$name"
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Save-Icon (Join-Path $dir 'ic_launcher.png') ([int](48 * $k)) 1.0
    Save-Icon (Join-Path $dir 'ic_launcher_round.png') ([int](48 * $k)) 1.0
    Save-Icon (Join-Path $dir 'ic_launcher_foreground.png') ([int](108 * $k)) $foregroundScale
    Write-Host "mipmap-$name"
}
$src.Dispose()
