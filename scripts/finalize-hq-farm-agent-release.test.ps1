$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'finalize-hq-farm-agent-release.ps1'
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("hq-farm-finalize-test-" + [guid]::NewGuid().ToString('N'))
$preVmpDirectory = Join-Path $tempRoot 'pre-vmp'
$vmpOutputDirectory = Join-Path $tempRoot 'vmp-output'
$preVmpExe = Join-Path $preVmpDirectory 'HQ Farm.exe'
$vmpOutputExe = Join-Path $vmpOutputDirectory 'HQ Farm.exe'

function Assert-Fails([scriptblock]$Action, [string]$Description) {
  $failed = $false
  try { & $Action } catch { $failed = $true }
  if (-not $failed) { throw "Expected finalizer to reject $Description." }
}

try {
  New-Item -ItemType Directory -Force -Path $preVmpDirectory, $vmpOutputDirectory | Out-Null
  [System.IO.File]::WriteAllBytes($preVmpExe, [byte[]](0x4d, 0x5a, 0x90, 0x00, 0x01))
  $hash = (Get-FileHash -LiteralPath $preVmpExe -Algorithm SHA256).Hash.ToLowerInvariant()
  $manifest = @{ agentId = 'hq-farm'; product = 'HQ Farm'; preVmp = @{ file = 'pre-vmp/HQ Farm.exe'; sha256 = $hash } } | ConvertTo-Json -Depth 5
  [System.IO.File]::WriteAllText((Join-Path $tempRoot 'PRE-VMP-MANIFEST.json'), $manifest, [System.Text.UTF8Encoding]::new($false))

  Assert-Fails { & $scriptPath -ReleaseDirectory $tempRoot } 'missing GUI output'
  [System.IO.File]::WriteAllBytes($vmpOutputExe, [System.IO.File]::ReadAllBytes($preVmpExe))
  Assert-Fails { & $scriptPath -ReleaseDirectory $tempRoot } 'unchanged GUI output'
  [System.IO.File]::WriteAllBytes($vmpOutputExe, [byte[]](0x6e, 0x6f, 0x74, 0x2d, 0x70, 0x65))
  Assert-Fails { & $scriptPath -ReleaseDirectory $tempRoot } 'non-PE GUI output'
  [System.IO.File]::WriteAllBytes($vmpOutputExe, [byte[]](0x4d, 0x5a, 0x90, 0x00, 0x02))
  & $scriptPath -ReleaseDirectory $tempRoot
  if ($null -ne $LASTEXITCODE -and $LASTEXITCODE -ne 0) { throw "Finalizer exited with $LASTEXITCODE." }
  foreach ($required in @('HQ Farm.exe', 'RELEASE-MANIFEST.json', 'SHA256SUMS.txt')) {
    if (-not (Test-Path -LiteralPath (Join-Path $tempRoot $required))) { throw "Finalizer did not write $required." }
  }
  $finalManifest = Get-Content -Raw -Encoding UTF8 (Join-Path $tempRoot 'RELEASE-MANIFEST.json')
  if ($finalManifest.Contains('Farm_Go') -or $finalManifest -notmatch 'HQ Farm\.exe') { throw 'Final manifest branding is invalid.' }
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
