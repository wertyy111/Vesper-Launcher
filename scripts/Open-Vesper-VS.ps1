param(
    [string]$SolutionPath
)

$ErrorActionPreference = 'Stop'

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
$dotnetRoot = Join-Path $env:LOCALAPPDATA 'VesperCodexTools\dotnet-sdk'
$userBuildRoot = Join-Path $env:LOCALAPPDATA 'VesperLauncherBuild'

if (-not (Test-Path (Join-Path $dotnetRoot 'dotnet.exe'))) {
    throw "Local .NET SDK was not found: $dotnetRoot. Install the SDK or ask Codex to reinstall it."
}

New-Item -ItemType Directory -Force -Path $userBuildRoot | Out-Null

$env:DOTNET_ROOT = $dotnetRoot
$env:DOTNET_CLI_HOME = Join-Path $repoRoot '.dotnet_home'
$env:NUGET_PACKAGES = Join-Path $repoRoot '.nuget'
$env:DOTNET_SKIP_FIRST_TIME_EXPERIENCE = '1'
$env:VESPER_USER_BUILD_ROOT = $userBuildRoot
$env:PATH = "$dotnetRoot;$env:PATH"

if ([string]::IsNullOrWhiteSpace($SolutionPath)) {
    $SolutionPath = Get-ChildItem -Path $repoRoot -Filter '*.sln' | Select-Object -First 1 -ExpandProperty FullName
}

$resolvedSolution = Resolve-Path $SolutionPath
$devenvCandidates = @(
    "${env:ProgramFiles}\Microsoft Visual Studio\2022\Community\Common7\IDE\devenv.exe",
    "${env:ProgramFiles}\Microsoft Visual Studio\2022\Professional\Common7\IDE\devenv.exe",
    "${env:ProgramFiles}\Microsoft Visual Studio\2022\Enterprise\Common7\IDE\devenv.exe",
    "${env:ProgramFiles(x86)}\Microsoft Visual Studio\2019\Community\Common7\IDE\devenv.exe",
    "${env:ProgramFiles(x86)}\Microsoft Visual Studio\2019\Professional\Common7\IDE\devenv.exe",
    "${env:ProgramFiles(x86)}\Microsoft Visual Studio\2019\Enterprise\Common7\IDE\devenv.exe"
) | Where-Object { $_ -and (Test-Path $_) }

if ($devenvCandidates.Count -gt 0) {
    Start-Process -FilePath $devenvCandidates[0] -ArgumentList "`"$resolvedSolution`""
    return
}

Start-Process -FilePath $resolvedSolution
