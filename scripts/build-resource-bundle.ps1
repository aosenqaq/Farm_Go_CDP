$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$root = Split-Path -Parent $PSScriptRoot
$source = Join-Path $root "resources\gameConfig"
$bundle = Join-Path $root "resources\gameConfig.bundle.zip"
$manifestName = ".farm-go-resource-manifest.json"

if (-not (Test-Path -LiteralPath $source -PathType Container)) {
  throw "Game config source directory is missing: $source"
}

$files = @(Get-ChildItem -LiteralPath $source -Recurse -File | Sort-Object FullName)
$manifest = [ordered]@{}
foreach ($file in $files) {
  $relative = $file.FullName.Substring($source.Length).TrimStart('\', '/') -replace '\\', '/'
  $manifest[$relative] = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
}

Remove-Item -LiteralPath $bundle -Force -ErrorAction SilentlyContinue
$archive = [System.IO.Compression.ZipFile]::Open($bundle, [System.IO.Compression.ZipArchiveMode]::Create)
try {
  foreach ($file in $files) {
    $relative = $file.FullName.Substring($source.Length).TrimStart('\', '/') -replace '\\', '/'
    [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile(
      $archive,
      $file.FullName,
      $relative,
      [System.IO.Compression.CompressionLevel]::Optimal
    ) | Out-Null
  }
  $entry = $archive.CreateEntry($manifestName, [System.IO.Compression.CompressionLevel]::Optimal)
  $writer = [System.IO.StreamWriter]::new($entry.Open(), [System.Text.UTF8Encoding]::new($false))
  try {
    $writer.Write((@{ files = $manifest } | ConvertTo-Json -Compress -Depth 3))
  } finally {
    $writer.Dispose()
  }
} finally {
  $archive.Dispose()
}

Write-Host "Generated $bundle with $($files.Count) files."
