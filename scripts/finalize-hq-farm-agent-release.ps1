param([Parameter(Mandatory = $true)][string]$ReleaseDirectory)

$ErrorActionPreference = 'Stop'

function Get-ForbiddenBranding([string[]]$Paths) {
  $matches = [System.Collections.Generic.List[string]]::new()
  $terms = @('Farm Go', 'Farm_Go', 'farm-go', 'farm_go', 'FarmGo', 'farmGo', 'FARM_GO')
  foreach ($path in $Paths) {
    $content = [System.IO.File]::ReadAllText($path, [System.Text.Encoding]::UTF8)
    foreach ($term in $terms) {
      if ($content.Contains($term)) { $matches.Add("$path contains $term") }
    }
  }
  return $matches
}

$releaseDirectoryPath = [System.IO.Path]::GetFullPath($ReleaseDirectory)
$manifestPath = Join-Path $releaseDirectoryPath 'PRE-VMP-MANIFEST.json'
$preVmpExe = Join-Path $releaseDirectoryPath 'pre-vmp\HQ Farm.exe'
$vmpOutputExe = Join-Path $releaseDirectoryPath 'vmp-output\HQ Farm.exe'
$finalExe = Join-Path $releaseDirectoryPath 'HQ Farm.exe'

if (-not (Test-Path -LiteralPath $manifestPath) -or -not (Test-Path -LiteralPath $preVmpExe)) {
  throw 'HQ Farm pre-VMP manifest or executable is missing.'
}
if (-not (Test-Path -LiteralPath $vmpOutputExe)) {
  throw "HQ Farm VMProtect output is missing: $vmpOutputExe"
}
if (Test-Path -LiteralPath $finalExe) {
  throw "HQ Farm final executable already exists: $finalExe"
}

$header = [System.IO.File]::ReadAllBytes($vmpOutputExe)
if ($header.Length -lt 2 -or $header[0] -ne 0x4d -or $header[1] -ne 0x5a) {
  throw 'HQ Farm VMProtect output is not a PE executable.'
}

$preManifest = Get-Content -Raw -Encoding UTF8 -LiteralPath $manifestPath | ConvertFrom-Json
$preVmpHash = [string]$preManifest.preVmp.sha256
$vmpOutputHash = (Get-FileHash -LiteralPath $vmpOutputExe -Algorithm SHA256).Hash.ToLowerInvariant()
if ([string]::IsNullOrWhiteSpace($preVmpHash) -or $vmpOutputHash -eq $preVmpHash.ToLowerInvariant()) {
  throw 'HQ Farm VMProtect output hash matches the pre-VMP executable.'
}

$preScan = Get-ForbiddenBranding @($manifestPath, $preVmpExe, $vmpOutputExe)
if ($preScan.Count -gt 0) { throw "HQ Farm finalization branding verification failed: $($preScan[0])" }

Copy-Item -LiteralPath $vmpOutputExe -Destination $finalExe
$finalHash = (Get-FileHash -LiteralPath $finalExe -Algorithm SHA256).Hash.ToLowerInvariant()
$releaseManifest = [ordered]@{
  agentId = $preManifest.agentId
  product = 'HQ Farm'
  version = $preManifest.version
  finalizedAt = (Get-Date).ToUniversalTime().ToString('o')
  preVmp = $preManifest.preVmp
  vmProtectOutput = [ordered]@{ file = 'vmp-output/HQ Farm.exe'; bytes = (Get-Item -LiteralPath $vmpOutputExe).Length; sha256 = $vmpOutputHash }
  final = [ordered]@{ file = 'HQ Farm.exe'; bytes = (Get-Item -LiteralPath $finalExe).Length; sha256 = $finalHash }
}
$releaseManifestPath = Join-Path $releaseDirectoryPath 'RELEASE-MANIFEST.json'
$checksumPath = Join-Path $releaseDirectoryPath 'SHA256SUMS.txt'
[System.IO.File]::WriteAllText($releaseManifestPath, ($releaseManifest | ConvertTo-Json -Depth 8), [System.Text.UTF8Encoding]::new($false))
[System.IO.File]::WriteAllText($checksumPath, "$finalHash  HQ Farm.exe`r`n", [System.Text.UTF8Encoding]::new($false))

$files = @($manifestPath, $preVmpExe, $vmpOutputExe, $finalExe, $releaseManifestPath, $checksumPath)
$postScan = Get-ForbiddenBranding $files
if ($postScan.Count -gt 0) { throw "HQ Farm finalization branding verification failed: $($postScan[0])" }
Write-Output "HQ Farm final VMP release: $finalExe"
