$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Drawing

$projectRoot = Split-Path -Parent $PSScriptRoot
$scriptPath = Join-Path $PSScriptRoot 'build-hq-farm-icon.ps1'
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("hq-farm-icon-test-" + [guid]::NewGuid().ToString('N'))
$sourceLogo = Join-Path $tempRoot 'logo.png'
$outputDirectory = Join-Path $tempRoot 'icons'

try {
  New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
  $fixture = [System.Drawing.Bitmap]::new(3840, 2160)
  try {
    $graphics = [System.Drawing.Graphics]::FromImage($fixture)
    try {
      $graphics.Clear([System.Drawing.Color]::Black)
      $font = [System.Drawing.Font]::new('Arial', 360, [System.Drawing.FontStyle]::Bold)
      try {
        $graphics.DrawString('HQ FARM', $font, [System.Drawing.Brushes]::White, 780, 850)
      } finally {
        $font.Dispose()
      }
    } finally {
      $graphics.Dispose()
    }
    $fixture.Save($sourceLogo, [System.Drawing.Imaging.ImageFormat]::Png)
  } finally {
    $fixture.Dispose()
  }

  & $scriptPath -SourceLogo $sourceLogo -OutputDirectory $outputDirectory
  if ($null -ne $LASTEXITCODE -and $LASTEXITCODE -ne 0) {
    throw "Icon builder exited with $LASTEXITCODE."
  }

  $appIconPath = Join-Path $outputDirectory 'appicon.png'
  $icoPath = Join-Path $outputDirectory 'icon.ico'
  if (-not (Test-Path -LiteralPath $appIconPath) -or -not (Test-Path -LiteralPath $icoPath)) {
    throw 'Icon builder did not write both icon assets.'
  }

  $appIcon = [System.Drawing.Image]::FromFile($appIconPath)
  try {
    if ($appIcon.Width -ne 1024 -or $appIcon.Height -ne 1024) {
      throw "Expected 1024x1024 PNG, got $($appIcon.Width)x$($appIcon.Height)."
    }
  } finally {
    $appIcon.Dispose()
  }

  $ico = [System.IO.File]::ReadAllBytes($icoPath)
  if ($ico.Length -lt 6 -or [BitConverter]::ToString($ico[0..3]) -ne '00-00-01-00') {
    throw 'Expected a Windows ICO header.'
  }
  if ([BitConverter]::ToUInt16($ico, 4) -ne 7) {
    throw 'Expected seven ICO image frames.'
  }
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
