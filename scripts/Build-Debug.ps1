param(
    [switch]$BuildFrontend
)

$ErrorActionPreference = 'Stop'

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
$dotnetRoot = Join-Path $env:LOCALAPPDATA 'VesperCodexTools\dotnet-sdk'
$userBuildRoot = Join-Path $env:LOCALAPPDATA 'VesperLauncherBuild'
$solutionPath = Get-ChildItem -Path $repoRoot -Filter '*.sln' | Select-Object -First 1 -ExpandProperty FullName

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

$skipFrontend = if ($BuildFrontend) { 'false' } else { 'true' }

& (Join-Path $dotnetRoot 'dotnet.exe') build $solutionPath `
    -c Debug `
    -nologo `
    -p:SkipFrontendBuild=$skipFrontend
