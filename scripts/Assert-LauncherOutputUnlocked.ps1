param(
  [Parameter(Mandatory = $true, Position = 0, ValueFromRemainingArguments = $true)]
  [string[]]$OutputPath
)

$ErrorActionPreference = 'Stop'

foreach ($candidate in $OutputPath) {
  if (-not (Test-Path -LiteralPath $candidate)) {
    continue
  }

  $stream = $null
  try {
    $stream = [System.IO.File]::Open(
      $candidate,
      [System.IO.FileMode]::Open,
      [System.IO.FileAccess]::ReadWrite,
      [System.IO.FileShare]::None)
  }
  catch {
    Write-Error "Close running Vesper Launcher before Release build. Locked file: $candidate"
    exit 1
  }
  finally {
    if ($null -ne $stream) {
      $stream.Dispose()
    }
  }
}
