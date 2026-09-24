$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("farm-go-build-output-redaction-" + [System.Guid]::NewGuid().ToString("N"))
$fixtureRoot = Join-Path $testRoot "fixture"
$fixtureScripts = Join-Path $fixtureRoot "scripts"
$fakeUserProfile = Join-Path $testRoot "profile"
$fakeWailsDirectory = Join-Path $fakeUserProfile "go\bin"
$secret = [System.Guid]::NewGuid().ToString("N")
$originalEnvironment = @{}
$environmentValues = @{
  "USERPROFILE" = $fakeUserProfile
  "KAUTH_PROGRAM_ID" = "test-program"
  "KAUTH_PROGRAM_SECRET" = $secret
  "KAUTH_MERCHANT_PUBLIC_KEY" = "test-public-key"
  "FARM_GO_VERSION_NO" = "1"
  "FARM_GO_VERSION_NAME" = "test-version"
}

try {
  New-Item -ItemType Directory -Force $fixtureScripts, $fakeWailsDirectory | Out-Null
  Copy-Item (Join-Path $projectRoot "scripts\build.ps1") (Join-Path $fixtureScripts "build.ps1")
  [System.IO.File]::WriteAllText((Join-Path $fixtureScripts "build-resource-bundle.ps1"), "`$ErrorActionPreference = 'Stop'", [System.Text.Encoding]::ASCII)
  Copy-Item (Join-Path $projectRoot "wails.json") (Join-Path $fixtureRoot "wails.json")

  $fakeWails = @"
@echo off
echo Wails Build Options: LDFlags: -X Farm_Go/internal/license.buildProgramSecret=$secret
echo Build complete
echo KnownStructs: benign stderr 1>&2
exit /b %FAKE_WAILS_EXIT_CODE%
"@
  [System.IO.File]::WriteAllText((Join-Path $fakeWailsDirectory "wails.cmd"), $fakeWails, [System.Text.Encoding]::ASCII)

  $fixtureWailsConfig = Join-Path $fixtureRoot "wails.json"
  $originalWailsConfig = [System.IO.File]::ReadAllBytes($fixtureWailsConfig)

  foreach ($name in $environmentValues.Keys) {
    $originalEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
    [Environment]::SetEnvironmentVariable($name, $environmentValues[$name], "Process")
  }

  function Invoke-FakeBuild([int]$expectedExitCode) {
    [Environment]::SetEnvironmentVariable("FAKE_WAILS_EXIT_CODE", $expectedExitCode, "Process")
    $previousErrorActionPreference = $ErrorActionPreference
    try {
      $ErrorActionPreference = "Continue"
      $output = & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $fixtureScripts "build.ps1") 2>&1
      $exitCode = $LASTEXITCODE
    } finally {
      $ErrorActionPreference = $previousErrorActionPreference
    }
    return @{
      ExitCode = $exitCode
      Output = ($output | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine
      WailsConfig = [System.IO.File]::ReadAllBytes($fixtureWailsConfig)
    }
  }

  $successfulBuild = Invoke-FakeBuild 0
  if ($successfulBuild.ExitCode -ne 0) {
    throw "Expected benign Wails stderr to retain a successful exit code."
  }
  if (-not $successfulBuild.Output.Contains("KnownStructs: benign stderr")) {
    throw "Build output did not retain benign Wails stderr."
  }

  $failedBuild = Invoke-FakeBuild 37
  if ($failedBuild.ExitCode -ne 37) {
    throw "Expected build failure exit code to be preserved."
  }
  if ($failedBuild.Output.Contains($secret)) {
    throw "Build output leaked a configured secret."
  }
  if ($failedBuild.Output -match "(?i)\bldflags\b") {
    throw "Build output leaked Wails LDFlags."
  }
  if (-not $failedBuild.Output.Contains("Build complete")) {
    throw "Build output did not retain benign Wails output."
  }
  if ($originalWailsConfig.Length -ne $failedBuild.WailsConfig.Length -or
      [System.BitConverter]::ToString($originalWailsConfig) -ne [System.BitConverter]::ToString($failedBuild.WailsConfig)) {
    throw "wails.json was not restored byte-for-byte."
  }
} finally {
  foreach ($name in $originalEnvironment.Keys) {
    [Environment]::SetEnvironmentVariable($name, $originalEnvironment[$name], "Process")
  }
  Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}
