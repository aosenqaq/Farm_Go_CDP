param(
  [string]$OutputDirectory = (Join-Path $env:USERPROFILE "Desktop"),
  [string]$TempDirectory = $env:TEMP,
  [ValidateRange(1, 20)][int]$LogCount = 5
)

$ErrorActionPreference = "Stop"
$sensitivePrefix = "K" + "AUTH_"
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$stagingDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("farm-go-frida-diagnostics-" + [guid]::NewGuid())
$archivePath = Join-Path $OutputDirectory "Farm-Go-Frida-Diagnostics-$timestamp.zip"

function Protect-DiagnosticText([string]$Text) {
  if ($null -eq $Text) {
    return ""
  }
  $pattern = "(?im)\b" + [regex]::Escape($sensitivePrefix) + "[A-Z0-9_]*\s*[=:].*$"
  return [regex]::Replace($Text, $pattern, "[redacted sensitive environment variable]")
}

function Write-DiagnosticFile([string]$Path, [string]$Text) {
  [System.IO.File]::WriteAllText($Path, (Protect-DiagnosticText $Text), [System.Text.UTF8Encoding]::new($false))
}

function Invoke-DiagnosticCommand([scriptblock]$Command) {
  try {
    return (& $Command 2>&1 | Out-String)
  } catch {
    return "command failed: $($_.Exception.Message)"
  }
}

try {
  New-Item -ItemType Directory -Force -Path $OutputDirectory, $stagingDirectory | Out-Null
  $logDirectory = Join-Path $stagingDirectory "frida-logs"
  New-Item -ItemType Directory -Force -Path $logDirectory | Out-Null

  $summary = @(
    "Farm Go Frida diagnostic bundle",
    "CollectedAt: $(Get-Date -Format o)",
    "Computer: $env:COMPUTERNAME",
    "PowerShell: $($PSVersionTable.PSVersion)",
    "LogsRequested: $LogCount"
  ) -join [Environment]::NewLine
  Write-DiagnosticFile (Join-Path $stagingDirectory "summary.txt") $summary

  $logs = @(Get-ChildItem -LiteralPath $TempDirectory -Filter "farm_go_frida_*.log" -File -ErrorAction SilentlyContinue |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First $LogCount)
  if ($logs.Count -eq 0) {
    Write-DiagnosticFile (Join-Path $logDirectory "no-frida-log.txt") "No Frida helper logs were found in $TempDirectory."
  } else {
    foreach ($log in $logs) {
      $content = Get-Content -LiteralPath $log.FullName -Raw -ErrorAction Stop
      Write-DiagnosticFile (Join-Path $logDirectory $log.Name) $content
    }
  }

  $pythonInfo = @(
    "Python command",
    (Invoke-DiagnosticCommand { Get-Command python -All | Select-Object CommandType, Name, Source, Version | Format-List }),
    "Python version",
    (Invoke-DiagnosticCommand { python --version }),
    "Frida version",
    (Invoke-DiagnosticCommand { python -c "import frida; print(frida.__version__)" })
  ) -join [Environment]::NewLine
  Write-DiagnosticFile (Join-Path $stagingDirectory "python-frida.txt") $pythonInfo

  $processes = Invoke-DiagnosticCommand {
    Get-CimInstance Win32_Process |
      Where-Object { $_.Name -match "^(Farm_Go|python|pythonw|WeChatAppEx)\.exe$" } |
      Select-Object ProcessId, ParentProcessId, Name, ExecutablePath |
      Format-List
  }
  Write-DiagnosticFile (Join-Path $stagingDirectory "related-processes.txt") $processes

  $events = Invoke-DiagnosticCommand {
    Get-WinEvent -FilterHashtable @{ LogName = "Application"; StartTime = (Get-Date).AddHours(-24) } |
      Where-Object {
        $_.ProviderName -match "Windows Error Reporting|Application Error" -and
        $_.Message -match "(?i)python|frida|farm_go|wechatappex"
      } |
      Select-Object -First 20 TimeCreated, ProviderName, Id, Message |
      Format-List
  }
  Write-DiagnosticFile (Join-Path $stagingDirectory "application-events.txt") $events

  Write-DiagnosticFile (Join-Path $stagingDirectory "README.txt") @"
Reproduce the Frida connection failure, then run this script immediately.
Send the resulting zip file to support. The bundle contains Frida helper logs,
Python and Frida version information, selected process metadata, and recent
related application crash events. It does not collect license configuration.
"@

  $sensitiveMatches = Get-ChildItem -LiteralPath $stagingDirectory -Recurse -File |
    Select-String -SimpleMatch -Pattern $sensitivePrefix -ErrorAction SilentlyContinue
  if ($sensitiveMatches) {
    throw "diagnostic bundle contains a sensitive environment variable marker"
  }

  Compress-Archive -Path (Join-Path $stagingDirectory "*") -DestinationPath $archivePath -Force
  Write-Output "Diagnostic bundle created: $archivePath"
} finally {
  Remove-Item -LiteralPath $stagingDirectory -Recurse -Force -ErrorAction SilentlyContinue
}
