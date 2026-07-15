param(
    [ValidateSet('Debug', 'Release')]
    [string]$Configuration = 'Release',

    [string[]]$RuntimeIdentifiers = @('linux-x64', 'osx-x64', 'osx-arm64'),

    [string]$Version,

    [string]$OutputRoot,

    [switch]$SelfContained,

    [switch]$BuildFrontend,

    [switch]$SkipProtection
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$projectName = if (Test-Path (Join-Path $repoRoot 'VesperLauncherApp')) { 'VesperLauncherApp' } else { 'VesperLauncher' }
$projectPath = Join-Path (Join-Path $repoRoot $projectName) 'VesperLauncher.CrossPlatform.csproj'
$outputRoot = if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    Join-Path $repoRoot '_build_verify/cross-platform-publish'
}
elseif ([System.IO.Path]::IsPathRooted($OutputRoot)) {
    [System.IO.Path]::GetFullPath($OutputRoot)
}
else {
    [System.IO.Path]::GetFullPath((Join-Path $repoRoot $OutputRoot))
}
$workRoot = if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    Join-Path $repoRoot '_build_verify'
}
else {
    Join-Path $outputRoot '.work'
}
$nugetRoot = if ([string]::IsNullOrWhiteSpace($env:NUGET_PACKAGES)) {
    Join-Path $workRoot 'nuget-packages'
}
else {
    [System.IO.Path]::GetFullPath($env:NUGET_PACKAGES)
}
$temporaryRoot = Join-Path $workRoot 'tmp'
$httpCacheRoot = if ([string]::IsNullOrWhiteSpace($env:NUGET_HTTP_CACHE_PATH)) {
    Join-Path $workRoot 'nuget-http-cache'
}
else {
    [System.IO.Path]::GetFullPath($env:NUGET_HTTP_CACHE_PATH)
}
$dotnetHomeRoot = if ([string]::IsNullOrWhiteSpace($env:DOTNET_CLI_HOME)) {
    Join-Path $workRoot 'dotnet-home'
}
else {
    [System.IO.Path]::GetFullPath($env:DOTNET_CLI_HOME)
}
$userBuildRoot = Join-Path $workRoot 'user-build-root'

if (-not (Test-Path $projectPath)) {
    throw "Cross-platform project was not found: $projectPath"
}

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
New-Item -ItemType Directory -Force -Path $nugetRoot | Out-Null
New-Item -ItemType Directory -Force -Path $temporaryRoot | Out-Null
New-Item -ItemType Directory -Force -Path $httpCacheRoot | Out-Null
New-Item -ItemType Directory -Force -Path $dotnetHomeRoot | Out-Null
New-Item -ItemType Directory -Force -Path $userBuildRoot | Out-Null

$env:TMP = $temporaryRoot
$env:TEMP = $temporaryRoot
$env:NUGET_PACKAGES = $nugetRoot
$env:NUGET_HTTP_CACHE_PATH = $httpCacheRoot
$env:DOTNET_CLI_HOME = $dotnetHomeRoot
$env:VESPER_USER_BUILD_ROOT = $userBuildRoot

foreach ($rid in $RuntimeIdentifiers) {
    if ($rid -notin @('linux-x64', 'osx-x64', 'osx-arm64')) {
        throw "Unsupported runtime identifier: $rid"
    }

    $publishDirectory = Join-Path $outputRoot $rid
    New-Item -ItemType Directory -Force -Path $publishDirectory | Out-Null

    $arguments = @(
        'publish',
        $projectPath,
        '-c', $Configuration,
        '-r', $rid,
        '--self-contained', $SelfContained.IsPresent.ToString().ToLowerInvariant(),
        "-p:RestorePackagesPath=$nugetRoot",
        "-p:SkipFrontendBuild=$(-not $BuildFrontend)",
        '-o', $publishDirectory,
        '-nologo'
    )

    if (-not [string]::IsNullOrWhiteSpace($Version)) {
        $safeVersion = $Version.Trim().TrimStart('v')
        if ($safeVersion -notmatch '^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$') {
            throw "Version must be a valid semantic version: $Version"
        }

        $arguments += "-p:Version=$safeVersion"
    }

    Write-Host "Publishing Vesper Launcher shell for $rid..."
    & dotnet @arguments

    if ($LASTEXITCODE -ne 0) {
        throw "Publish failed for $rid with exit code $LASTEXITCODE."
    }

    $frontendDist = Join-Path $repoRoot "$projectName/UserInterface/dist"
    $publishedFrontend = Join-Path $publishDirectory 'UserInterface/dist'
    if (-not (Test-Path -LiteralPath (Join-Path $frontendDist 'index.html') -PathType Leaf)) {
        throw "Frontend build output is missing: $frontendDist"
    }

    New-Item -ItemType Directory -Force -Path $publishedFrontend | Out-Null
    Copy-Item -Path (Join-Path $frontendDist '*') -Destination $publishedFrontend -Recurse -Force

    if ($Configuration -eq 'Release' -and -not $SkipProtection) {
        & (Join-Path $PSScriptRoot 'Protect-ReleaseBinaries.ps1') -PublishDirectory $publishDirectory
        if ($LASTEXITCODE -ne 0) {
            throw "Release protection failed for $rid with exit code $LASTEXITCODE."
        }
    }
}

Write-Host "Cross-platform shell publish completed: $outputRoot"

