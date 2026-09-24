param(
  [Parameter(Mandatory = $true)][string]$SourceLogo,
  [Parameter(Mandatory = $true)][string]$OutputDirectory
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Drawing

function Get-PngBytes([System.Drawing.Image]$image, [int]$size) {
  $frame = [System.Drawing.Bitmap]::new($size, $size)
  try {
    $graphics = [System.Drawing.Graphics]::FromImage($frame)
    try {
      $graphics.Clear([System.Drawing.Color]::Black)
      $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
      $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
      $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
      $margin = [math]::Round($size * 0.05)
      $availableWidth = $size - (2 * $margin)
      $availableHeight = $size - (2 * $margin)
      $scale = [math]::Min($availableWidth / $image.Width, $availableHeight / $image.Height)
      $width = [math]::Round($image.Width * $scale)
      $height = [math]::Round($image.Height * $scale)
      $left = [math]::Round(($size - $width) / 2)
      $top = [math]::Round(($size - $height) / 2)
      $graphics.DrawImage($image, [System.Drawing.Rectangle]::new($left, $top, $width, $height))
    } finally {
      $graphics.Dispose()
    }

    $stream = [System.IO.MemoryStream]::new()
    try {
      $frame.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
      Write-Output -NoEnumerate $stream.ToArray()
    } finally {
      $stream.Dispose()
    }
  } finally {
    $frame.Dispose()
  }
}

function Write-Ico([string]$Path, [System.Collections.Generic.List[byte[]]]$Frames, [int[]]$Sizes) {
  $stream = [System.IO.MemoryStream]::new()
  $writer = [System.IO.BinaryWriter]::new($stream)
  try {
    $writer.Write([uint16]0)
    $writer.Write([uint16]1)
    $writer.Write([uint16]$Frames.Count)
    $offset = 6 + (16 * $Frames.Count)
    for ($index = 0; $index -lt $Frames.Count; $index++) {
      $sizeByte = if ($Sizes[$index] -ge 256) { [byte]0 } else { [byte]$Sizes[$index] }
      $writer.Write($sizeByte)
      $writer.Write($sizeByte)
      $writer.Write([byte]0)
      $writer.Write([byte]0)
      $writer.Write([uint16]1)
      $writer.Write([uint16]32)
      $writer.Write([uint32]$Frames[$index].Length)
      $writer.Write([uint32]$offset)
      $offset += $Frames[$index].Length
    }
    foreach ($frame in $Frames) {
      $writer.Write($frame)
    }
    [System.IO.File]::WriteAllBytes($Path, $stream.ToArray())
  } finally {
    $writer.Dispose()
    $stream.Dispose()
  }
}

$sourcePath = [System.IO.Path]::GetFullPath($SourceLogo)
if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
  throw "HQ Farm source logo is missing: $sourcePath"
}

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$outputPath = [System.IO.Path]::GetFullPath($OutputDirectory)
$source = $null
try {
  $source = [System.Drawing.Image]::FromFile($sourcePath)
  if ($source.Width -le 0 -or $source.Height -le 0) {
    throw 'HQ Farm source logo has invalid dimensions.'
  }

  $png = Get-PngBytes $source 1024
  $appIconPath = Join-Path $outputPath 'appicon.png'
  [System.IO.File]::WriteAllBytes($appIconPath, $png)

  $sizes = [int[]](16, 24, 32, 48, 64, 128, 256)
  $frames = [System.Collections.Generic.List[byte[]]]::new()
  foreach ($size in $sizes) {
    $frames.Add((Get-PngBytes $source $size))
  }
  $icoPath = Join-Path $outputPath 'icon.ico'
  Write-Ico $icoPath $frames $sizes

  $generated = [System.Drawing.Image]::FromFile($appIconPath)
  try {
    if ($generated.Width -ne 1024 -or $generated.Height -ne 1024) {
      throw 'HQ Farm app icon did not render as a square PNG.'
    }
  } finally {
    $generated.Dispose()
  }
  $ico = [System.IO.File]::ReadAllBytes($icoPath)
  if ($ico.Length -lt 6 -or [BitConverter]::ToString($ico[0..3]) -ne '00-00-01-00' -or [BitConverter]::ToUInt16($ico, 4) -ne 7) {
    throw 'HQ Farm ICO validation failed.'
  }
} finally {
  if ($null -ne $source) {
    $source.Dispose()
  }
}
