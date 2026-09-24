[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)]
  [int]$FarmGoProcessId,
  [ValidateRange(1, 10000)]
  [int]$Samples = 12,
  [ValidateRange(1, 3600)]
  [int]$IntervalSeconds = 5,
  [string]$OutputPath = ".\land-memory.csv"
)

function Get-DescendantProcessIds {
  param([int]$RootProcessId)

  $processes = Get-CimInstance Win32_Process
  $childrenByParent = @{}
  foreach ($process in $processes) {
    $parentId = [int]$process.ParentProcessId
    if (-not $childrenByParent.ContainsKey($parentId)) {
      $childrenByParent[$parentId] = @()
    }
    $childrenByParent[$parentId] += [int]$process.ProcessId
  }

  $result = [System.Collections.Generic.HashSet[int]]::new()
  $pending = [System.Collections.Generic.Queue[int]]::new()
  $pending.Enqueue($RootProcessId)
  while ($pending.Count -gt 0) {
    $parentId = $pending.Dequeue()
    foreach ($childId in ($childrenByParent[$parentId] | ForEach-Object { $_ })) {
      if ($result.Add($childId)) {
        $pending.Enqueue($childId)
      }
    }
  }
  return ,$result
}

$records = @()
for ($sample = 1; $sample -le $Samples; $sample++) {
  $descendants = Get-DescendantProcessIds -RootProcessId $FarmGoProcessId
  $webviewProcessesInfo = @(
    Get-CimInstance Win32_Process |
      Where-Object {
        $descendants.Contains([int]$_.ProcessId) -and
        $_.Name -eq "msedgewebview2.exe"
      }
  )
  $webviewIds = @($webviewProcessesInfo | Select-Object -ExpandProperty ProcessId)
  $rendererIds = @(
    $webviewProcessesInfo |
      Where-Object { $_.CommandLine -match "--type=renderer" } |
      Select-Object -ExpandProperty ProcessId
  )
  $rootProcess = Get-Process -Id $FarmGoProcessId -ErrorAction Stop
  $webviewProcesses = @($webviewIds | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
  $renderers = @($rendererIds | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
  if ($renderers.Count -eq 0) {
    throw "No WebView2 renderer was found below Farm_Go process $FarmGoProcessId."
  }
  $records += [pscustomobject]@{
    sample                    = $sample
    timestamp                 = (Get-Date).ToString("o")
    rootPrivateBytes          = [int64]$rootProcess.PrivateMemorySize64
    rootWorkingSetBytes       = [int64]$rootProcess.WorkingSet64
    rootHandles               = [int]$rootProcess.HandleCount
    rootThreads               = [int]$rootProcess.Threads.Count
    rendererCount             = $renderers.Count
    privateBytes              = [int64](($renderers | Measure-Object -Property PrivateMemorySize64 -Sum).Sum)
    rendererWorkingSetBytes   = [int64](($renderers | Measure-Object -Property WorkingSet64 -Sum).Sum)
    rendererHandles           = [int](($renderers | Measure-Object -Property HandleCount -Sum).Sum)
    rendererThreads           = [int](($renderers | ForEach-Object { $_.Threads.Count } | Measure-Object -Sum).Sum)
    webviewProcessCount       = $webviewProcesses.Count
    totalPrivateBytes         = [int64]($rootProcess.PrivateMemorySize64 + ($webviewProcesses | Measure-Object -Property PrivateMemorySize64 -Sum).Sum)
    totalWorkingSetBytes      = [int64]($rootProcess.WorkingSet64 + ($webviewProcesses | Measure-Object -Property WorkingSet64 -Sum).Sum)
  }
  if ($sample -lt $Samples) {
    Start-Sleep -Seconds $IntervalSeconds
  }
}

$records | Export-Csv -NoTypeInformation -Encoding utf8 -Path $OutputPath
Write-Output "Wrote $Samples renderer memory samples to $OutputPath"
