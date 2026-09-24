param([switch]$ProtectedFrontend)

$ErrorActionPreference = "Stop"

Push-Location $PSScriptRoot\..
try {
  $env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"
  & (Join-Path $PSScriptRoot "build-resource-bundle.ps1")

  $wailsConfigPath = Join-Path (Get-Location) "wails.json"
  $originalWailsConfig = [System.IO.File]::ReadAllBytes($wailsConfigPath)
  $wailsExitCode = 0
  try {
    $wailsConfig = Get-Content -Raw $wailsConfigPath | ConvertFrom-Json
    $versionName = [Environment]::GetEnvironmentVariable("FARM_GO_VERSION_NAME")
    if (-not [string]::IsNullOrWhiteSpace($versionName)) {
      $wailsConfig.info.productVersion = $versionName
    }
    if ($ProtectedFrontend) {
      $wailsConfig.'frontend:build' = 'npm run build:protected'
    }
    $updatedWailsConfig = $wailsConfig | ConvertTo-Json -Depth 10
    [System.IO.File]::WriteAllText($wailsConfigPath, $updatedWailsConfig, [System.Text.UTF8Encoding]::new($false))

    Write-Host "Building Farm_Go"
    $previousErrorActionPreference = $ErrorActionPreference
    try {
      $ErrorActionPreference = "Continue"
      $wailsOutput = & wails build -clean -trimpath 2>&1
      $wailsExitCode = $LASTEXITCODE
    } finally {
      $ErrorActionPreference = $previousErrorActionPreference
    }
    foreach ($line in $wailsOutput) {
      $text = $line.ToString()
      if ($text -notmatch "(?i)\bldflags\b" -and -not $text.Contains($env:KAUTH_PROGRAM_SECRET)) {
        Write-Output $text
      }
    }
  } finally {
    [System.IO.File]::WriteAllBytes($wailsConfigPath, $originalWailsConfig)
  }
  if ($wailsExitCode -ne 0) {
    exit $wailsExitCode
  }
} finally {
  Pop-Location
}
