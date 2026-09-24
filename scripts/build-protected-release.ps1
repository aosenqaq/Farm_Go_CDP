param(
  [Parameter(Mandatory = $true)][string]$Version,
  [string]$ReleaseId = "",
  [string]$ReleaseRoot = ""
)

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($ReleaseId)) {
  $ReleaseId = "farm-go-v$Version-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
}
if ($ReleaseId -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') {
  throw "ReleaseId contains unsupported characters."
}
if ([string]::IsNullOrWhiteSpace($ReleaseRoot)) {
  $ReleaseRoot = Join-Path $projectRoot "release"
}

$releaseRootPath = [System.IO.Path]::GetFullPath($ReleaseRoot)
$releaseDirectory = Join-Path $releaseRootPath $ReleaseId
if (Test-Path -LiteralPath $releaseDirectory) {
  throw "Release directory already exists: $releaseDirectory"
}

& (Join-Path $PSScriptRoot "build.ps1") -ProtectedFrontend
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

$buildExe = Join-Path $projectRoot "build\bin\Farm_Go.exe"
if (-not (Test-Path -LiteralPath $buildExe)) {
  throw "Protected Wails build did not produce $buildExe"
}

$preVmpDirectory = Join-Path $releaseDirectory "pre-vmp"
$vmpOutputDirectory = Join-Path $releaseDirectory "vmp-output"
New-Item -ItemType Directory -Force -Path $preVmpDirectory, $vmpOutputDirectory | Out-Null
$preVmpExe = Join-Path $preVmpDirectory "Farm_Go.exe"
Copy-Item -LiteralPath $buildExe -Destination $preVmpExe

$assetRoot = Join-Path $projectRoot "frontend\dist\assets"
if (-not (Test-Path -LiteralPath $assetRoot)) {
  throw "Protected frontend assets directory is missing: $assetRoot"
}
$frontendDistRoot = Join-Path $projectRoot "frontend\dist"
$frontendAssets = @(Get-ChildItem -LiteralPath $assetRoot -Recurse -File -Filter *.js |
  Sort-Object FullName |
  ForEach-Object {
    [ordered]@{
      path = $_.FullName.Substring($frontendDistRoot.Length).TrimStart('\', '/').Replace('\', '/')
      bytes = $_.Length
      sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    }
  })
if ($frontendAssets.Count -eq 0) {
  throw "Protected frontend build did not produce JavaScript assets."
}

$preVmpHash = (Get-FileHash -LiteralPath $preVmpExe -Algorithm SHA256).Hash.ToLowerInvariant()
$manifest = [ordered]@{
  releaseId = $ReleaseId
  version = $Version
  createdAt = (Get-Date).ToUniversalTime().ToString("o")
  preVmp = [ordered]@{
    file = "pre-vmp/Farm_Go.exe"
    bytes = (Get-Item -LiteralPath $preVmpExe).Length
    sha256 = $preVmpHash
  }
  frontendAssets = $frontendAssets
}
[System.IO.File]::WriteAllText(
  (Join-Path $releaseDirectory "PRE-VMP-MANIFEST.json"),
  ($manifest | ConvertTo-Json -Depth 8),
  [System.Text.UTF8Encoding]::new($false)
)

$checklist = @"
# VMProtect GUI Checklist

Input: ``pre-vmp/Farm_Go.exe``
Output: ``vmp-output/Farm_Go.exe``

Use VMProtect Professional GUI. This installation does not provide a supported headless CLI.

## Initial Settings

- Protect ``EntryPoint`` with Super (mutation + virtualization), maximum supported complexity.
- Enable memory protection, import protection, resource protection, compressed output, and remove debug information.
- Enable user-mode debugger detection only.
- Keep kernel-mode debugger detection, serial-number lock, and Shadow Stack compatibility disabled.

## Priority Code Areas

1. ``internal/runtime/cdp/*``, ``internal/runtime/wmpf/*``, ``internal/runtime/qqpatch/*``
3. ``internal/farm/automation/*``, ``internal/farm/social/*``, ``internal/farm/stealrules/*``

Go symbols may not be visible after ``-trimpath``. Apply the initial profile to ``EntryPoint`` and the global options above; do not attempt to alter the application's embedded JavaScript resources.

After protection, start the application manually before running ``finalize-vmp-release.ps1``.
"@
[System.IO.File]::WriteAllText((Join-Path $releaseDirectory "VMP-GUI-CHECKLIST.md"), $checklist, [System.Text.UTF8Encoding]::new($false))
New-Item -ItemType File -Force -Path (Join-Path $vmpOutputDirectory ".gitkeep") | Out-Null

Write-Output "Pre-VMP release created: $releaseDirectory"
