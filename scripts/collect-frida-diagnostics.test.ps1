$ErrorActionPreference = "Stop"

$root = Join-Path ([System.IO.Path]::GetTempPath()) ("farm-go-frida-diagnostics-test-" + [guid]::NewGuid())
try {
  $tempDirectory = Join-Path $root "temp"
  $outputDirectory = Join-Path $root "output"
  New-Item -ItemType Directory -Force -Path $tempDirectory, $outputDirectory | Out-Null
  Set-Content -LiteralPath (Join-Path $tempDirectory "farm_go_frida_12345.log") -Value "FRIDA_ERROR test failure" -NoNewline

  & (Join-Path $PSScriptRoot "collect-frida-diagnostics.ps1") -TempDirectory $tempDirectory -OutputDirectory $outputDirectory -LogCount 1
  if ($LASTEXITCODE -ne 0) {
    throw "diagnostic collector failed"
  }

  $archive = Get-ChildItem -LiteralPath $outputDirectory -Filter "Farm-Go-Frida-Diagnostics-*.zip" -File | Select-Object -First 1
  if ($null -eq $archive) {
    throw "diagnostic archive was not created"
  }

  $expanded = Join-Path $root "expanded"
  Expand-Archive -LiteralPath $archive.FullName -DestinationPath $expanded
  if (-not (Test-Path -LiteralPath (Join-Path $expanded "frida-logs\farm_go_frida_12345.log"))) {
    throw "diagnostic archive did not include the Frida helper log"
  }
  if (-not (Test-Path -LiteralPath (Join-Path $expanded "summary.txt"))) {
    throw "diagnostic archive did not include the summary"
  }
  if (Get-ChildItem -LiteralPath $expanded -Recurse -File | Select-String -SimpleMatch -Pattern "KAUTH_") {
    throw "diagnostic archive contains a forbidden KAUTH marker"
  }

  Write-Output "collect-frida-diagnostics test passed"
} finally {
  Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}
