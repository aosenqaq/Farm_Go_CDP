param(
  [string]$SourcePython = "python"
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$root = Split-Path -Parent $PSScriptRoot
$bundle = Join-Path $root "resources\frida-python.bundle.zip"
$work = Join-Path ([System.IO.Path]::GetTempPath()) ("farm-go-frida-python-" + [guid]::NewGuid())
$runtime = Join-Path $work "runtime"

try {
  New-Item -ItemType Directory -Force -Path $work, $runtime | Out-Null
  $sourceCommand = Get-Command $SourcePython -CommandType Application -ErrorAction Stop | Select-Object -First 1
  $sourceRoot = Split-Path -Parent $sourceCommand.Source
  if (-not (Test-Path -LiteralPath (Join-Path $sourceRoot "python.exe"))) {
    throw "Source Python executable is missing: $($sourceCommand.Source)"
  }
  foreach ($name in @("python.exe", "pythonw.exe", "python3.dll", "python313.dll", "vcruntime140.dll", "vcruntime140_1.dll", "LICENSE.txt")) {
    $source = Join-Path $sourceRoot $name
    if (Test-Path -LiteralPath $source) {
      Copy-Item -LiteralPath $source -Destination (Join-Path $runtime $name)
    }
  }
  Copy-Item -LiteralPath (Join-Path $sourceRoot "DLLs") -Destination (Join-Path $runtime "DLLs") -Recurse
  Copy-Item -LiteralPath (Join-Path $sourceRoot "Lib") -Destination (Join-Path $runtime "Lib") -Recurse
  Remove-Item -LiteralPath (Join-Path $runtime "Lib\site-packages") -Recurse -Force -ErrorAction SilentlyContinue

  $python = Join-Path $runtime "python.exe"
  $sourceSitePackages = Join-Path $sourceRoot "Lib\site-packages"
  $destinationSitePackages = Join-Path $runtime "Lib\site-packages"
  New-Item -ItemType Directory -Force -Path $destinationSitePackages | Out-Null
  Copy-Item -LiteralPath (Join-Path $sourceSitePackages "frida") -Destination (Join-Path $destinationSitePackages "frida") -Recurse
  $fridaMetadata = Get-ChildItem -LiteralPath $sourceSitePackages -Filter "frida-*.dist-info" -Directory | Select-Object -First 1
  if ($null -eq $fridaMetadata) { throw "Source Python does not have Frida metadata." }
  Copy-Item -LiteralPath $fridaMetadata.FullName -Destination (Join-Path $destinationSitePackages $fridaMetadata.Name) -Recurse
  & $python -c "import frida; print(frida.__version__)"
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

  $files = @(Get-ChildItem -LiteralPath $runtime -Recurse -File | Sort-Object FullName)
  $manifest = [ordered]@{}
  foreach ($file in $files) {
    $relative = $file.FullName.Substring($runtime.Length).TrimStart('\', '/') -replace '\\', '/'
    $manifest[$relative] = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
  }
  $metadata = [ordered]@{
    pythonVersion = (& $python -c "import sys; print(sys.version)").Trim()
    fridaVersion = (& $python -c "import frida; print(frida.__version__)").Trim()
    files = $manifest
  } | ConvertTo-Json -Compress -Depth 4
  [System.IO.File]::WriteAllText((Join-Path $runtime ".farm-go-frida-runtime-manifest.json"), $metadata, [System.Text.UTF8Encoding]::new($false))

  Remove-Item -LiteralPath $bundle -Force -ErrorAction SilentlyContinue
  $archive = [System.IO.Compression.ZipFile]::Open($bundle, [System.IO.Compression.ZipArchiveMode]::Create)
  try {
    Get-ChildItem -LiteralPath $runtime -Recurse -File | ForEach-Object {
      $relative = $_.FullName.Substring($runtime.Length).TrimStart('\', '/') -replace '\\', '/'
      [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $_.FullName, $relative, [System.IO.Compression.CompressionLevel]::Optimal) | Out-Null
    }
  } finally {
    $archive.Dispose()
  }
  Write-Output "Generated $bundle"
} finally {
  Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}
