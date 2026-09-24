param([Parameter(Mandatory = $true)][string]$ReleaseDirectory)

$ErrorActionPreference = "Stop"
$releaseDirectory = [System.IO.Path]::GetFullPath($ReleaseDirectory)
$manifestPath = Join-Path $releaseDirectory "PRE-VMP-MANIFEST.json"
$preVmpExe = Join-Path $releaseDirectory "pre-vmp\Farm_Go.exe"
$vmpOutputExe = Join-Path $releaseDirectory "vmp-output\Farm_Go.exe"
$finalExe = Join-Path $releaseDirectory "Farm_Go.exe"

if (-not (Test-Path -LiteralPath $manifestPath) -or -not (Test-Path -LiteralPath $preVmpExe)) {
  throw "Pre-VMP manifest or executable is missing."
}
if (-not (Test-Path -LiteralPath $vmpOutputExe)) {
  throw "VMProtect GUI output is missing: $vmpOutputExe"
}
if (Test-Path -LiteralPath $finalExe) {
  throw "Final executable already exists: $finalExe"
}

$header = [System.IO.File]::ReadAllBytes($vmpOutputExe)
if ($header.Length -lt 2 -or $header[0] -ne 0x4d -or $header[1] -ne 0x5a) {
  throw "VMProtect GUI output is not a PE executable."
}

$preManifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$preVmpHash = [string]$preManifest.preVmp.sha256
$vmpOutputHash = (Get-FileHash -LiteralPath $vmpOutputExe -Algorithm SHA256).Hash.ToLowerInvariant()
if ([string]::IsNullOrWhiteSpace($preVmpHash) -or $vmpOutputHash -eq $preVmpHash.ToLowerInvariant()) {
  throw "VMProtect GUI output hash matches the pre-VMP executable."
}

Copy-Item -LiteralPath $vmpOutputExe -Destination $finalExe
$finalHash = (Get-FileHash -LiteralPath $finalExe -Algorithm SHA256).Hash.ToLowerInvariant()
$releaseManifest = [ordered]@{
  releaseId = $preManifest.releaseId
  version = $preManifest.version
  finalizedAt = (Get-Date).ToUniversalTime().ToString("o")
  preVmp = $preManifest.preVmp
  frontendAssets = $preManifest.frontendAssets
  vmProtectOutput = [ordered]@{
    file = "vmp-output/Farm_Go.exe"
    bytes = (Get-Item -LiteralPath $vmpOutputExe).Length
    sha256 = $vmpOutputHash
  }
  final = [ordered]@{
    file = "Farm_Go.exe"
    bytes = (Get-Item -LiteralPath $finalExe).Length
    sha256 = $finalHash
  }
}
[System.IO.File]::WriteAllText(
  (Join-Path $releaseDirectory "RELEASE-MANIFEST.json"),
  ($releaseManifest | ConvertTo-Json -Depth 10),
  [System.Text.UTF8Encoding]::new($false)
)
[System.IO.File]::WriteAllText(
  (Join-Path $releaseDirectory "SHA256SUMS.txt"),
  "$finalHash  Farm_Go.exe`r`n",
  [System.Text.UTF8Encoding]::new($false)
)

Write-Output "Final VMP release created: $finalExe"
