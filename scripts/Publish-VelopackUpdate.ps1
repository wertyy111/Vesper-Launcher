[CmdletBinding()]
param(
    [string]$Version,
    [string]$ProjectPath = 'MinecraftCheatLauncher/MinecraftCheatLauncher.csproj',
    [string]$RuntimeIdentifier = 'win-x64',
    [string]$Configuration = 'Release',
    [string]$Channel = 'win',
    [string]$PackId = 'Vesper.Launcher',
    [string]$PackTitle = 'Vesper Launcher',
    [string]$PackAuthors = 'Vesper Launcher',
    [string]$MainExe = 'MinecraftCheatLauncher.exe',
    [string]$SetupFileName = 'VesperLauncherSetup.exe',
    [string]$RepoUrl = 'https://github.com/wertyy111/Updates-Vesper',
    [string]$OutputRoot = '%LOCALAPPDATA%/VesperLauncherBuild/velopack',
    [string]$ReleaseNotesPath,
    [switch]$SkipFrontendBuild,
    [switch]$SkipPreviousDownload,
    [switch]$UploadDraft,
    [switch]$Publish
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Resolve-NormalizedPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    return [System.IO.Path]::GetFullPath([Environment]::ExpandEnvironmentVariables($Path))
}

function Resolve-RepoRelativePath {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][string]$Path
    )

    $expanded = [Environment]::ExpandEnvironmentVariables($Path)
    if ([System.IO.Path]::IsPathRooted($expanded)) {
        return [System.IO.Path]::GetFullPath($expanded)
    }

    return [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $expanded))
}

function Get-ProjectVersion {
    param([Parameter(Mandatory = $true)][string]$ProjectFile)

    [xml]$projectXml = Get-Content -LiteralPath $ProjectFile
    $versionNode = $projectXml.SelectSingleNode('/Project/PropertyGroup/Version')
    $projectVersion = if ($null -eq $versionNode) { $null } else { $versionNode.InnerText }
    if ([string]::IsNullOrWhiteSpace($projectVersion)) {
        throw "Version is missing in $ProjectFile."
    }

    return $projectVersion.Trim()
}

function Invoke-RepoDotnet {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    Push-Location $RepoRoot
    try {
        & dotnet @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "dotnet $($Arguments -join ' ') failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}

function Ensure-VpkTokenForUpload {
    if (-not [string]::IsNullOrWhiteSpace($env:VPK_TOKEN)) {
        return
    }

    if (-not [string]::IsNullOrWhiteSpace($env:GITHUB_TOKEN)) {
        $env:VPK_TOKEN = $env:GITHUB_TOKEN
        return
    }

    throw @(
        'GitHub token is required only for upload/publish.'
        'Set it in the current PowerShell session, not in source code:'
        '$env:VPK_TOKEN = "YOUR_FINE_GRAINED_TOKEN"'
        'Use a token scoped only to the binary updates repository.'
    ) -join [Environment]::NewLine
}

function Rename-SetupPackage {
    param(
        [Parameter(Mandatory = $true)][string]$ReleasesDirectory,
        [Parameter(Mandatory = $true)][string]$SetupFileName
    )

    $safeSetupFileName = [System.IO.Path]::GetFileName($SetupFileName)
    if ([string]::IsNullOrWhiteSpace($safeSetupFileName) -or $safeSetupFileName -ne $SetupFileName) {
        throw 'SetupFileName must be a file name without directory separators.'
    }

    $targetPath = Join-Path $ReleasesDirectory $safeSetupFileName
    $setupCandidates = Get-ChildItem -LiteralPath $ReleasesDirectory -Filter '*-Setup.exe' -File |
        Sort-Object LastWriteTimeUtc -Descending

    $source = $setupCandidates | Where-Object {
        -not [string]::Equals($_.FullName, $targetPath, [System.StringComparison]::OrdinalIgnoreCase)
    } | Select-Object -First 1

    if ($null -eq $source) {
        if (Test-Path -LiteralPath $targetPath -PathType Leaf) {
            Write-Host "Setup package already uses stable name: $safeSetupFileName"
            return
        }

        throw "Velopack setup package was not found in $ReleasesDirectory."
    }

    if (Test-Path -LiteralPath $targetPath -PathType Leaf) {
        Remove-Item -LiteralPath $targetPath -Force
    }

    Move-Item -LiteralPath $source.FullName -Destination $targetPath -Force
    Write-Host "Stable setup package: $targetPath"
}

$repoRoot = Resolve-NormalizedPath (Join-Path $PSScriptRoot '..')
$projectFullPath = Resolve-NormalizedPath (Join-Path $repoRoot $ProjectPath)
if (-not (Test-Path -LiteralPath $projectFullPath -PathType Leaf)) {
    throw "Project not found: $projectFullPath"
}

$userBuildRoot = Join-Path $env:LOCALAPPDATA 'VesperLauncherBuild'
$env:DOTNET_CLI_HOME = Join-Path $repoRoot '.dotnet_home'
$env:NUGET_PACKAGES = Join-Path $userBuildRoot 'nuget'
$env:DOTNET_SKIP_FIRST_TIME_EXPERIENCE = '1'
New-Item -ItemType Directory -Path $env:DOTNET_CLI_HOME, $env:NUGET_PACKAGES -Force | Out-Null

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = Get-ProjectVersion -ProjectFile $projectFullPath
}

$safeVersion = $Version.Trim().TrimStart('v')
if ([string]::IsNullOrWhiteSpace($safeVersion)) {
    throw 'Version must not be empty.'
}

$outputRootFullPath = Resolve-RepoRelativePath -RepoRoot $repoRoot -Path $OutputRoot
$releaseRoot = Join-Path $outputRootFullPath $safeVersion
$publishDirectory = Join-Path $releaseRoot "publish-$RuntimeIdentifier"
$releasesDirectory = Join-Path $releaseRoot 'Releases'
$iconPath = Resolve-NormalizedPath (Join-Path $repoRoot 'MinecraftCheatLauncher/Assets/vesper-app.ico')

if ($SkipPreviousDownload -and (Test-Path -LiteralPath $releasesDirectory)) {
    $resolvedReleasesDirectory = Resolve-NormalizedPath $releasesDirectory
    $resolvedOutputRoot = Resolve-NormalizedPath $outputRootFullPath
    if (-not $resolvedReleasesDirectory.StartsWith($resolvedOutputRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to clean unexpected Velopack releases directory: $resolvedReleasesDirectory"
    }

    Remove-Item -LiteralPath $resolvedReleasesDirectory -Recurse -Force
}

New-Item -ItemType Directory -Path $releaseRoot, $publishDirectory, $releasesDirectory -Force | Out-Null

Write-Host "Publishing Vesper Launcher $safeVersion for $RuntimeIdentifier..."
$publishArguments = @(
    'publish', $projectFullPath,
    '-c', $Configuration,
    '-r', $RuntimeIdentifier,
    '--self-contained', 'true',
    '-o', $publishDirectory,
    '-nologo',
    '-p:PublishSingleFile=false',
    '-p:DebugType=none',
    '-p:DebugSymbols=false',
    '-p:SkipLockedLauncherCheck=true',
    "-p:RestorePackagesPath=$env:NUGET_PACKAGES",
    "-p:SkipFrontendBuild=$($SkipFrontendBuild.IsPresent.ToString().ToLowerInvariant())"
)
Invoke-RepoDotnet -RepoRoot $repoRoot -Arguments $publishArguments

Write-Host "Restoring Velopack CLI..."
Invoke-RepoDotnet -RepoRoot $repoRoot -Arguments @('tool', 'restore')

if (-not $SkipPreviousDownload) {
    Write-Host "Trying to download previous Velopack release for delta generation..."
    try {
        Invoke-RepoDotnet -RepoRoot $repoRoot -Arguments @(
            'tool', 'run', 'vpk', '--',
            'download', 'github',
            '--outputDir', $releasesDirectory,
            '--channel', $Channel,
            '--repoUrl', $RepoUrl
        )
    }
    catch {
        Write-Warning "Previous release download was skipped: $($_.Exception.Message)"
    }
}

Write-Host "Packing Velopack release..."
$packArguments = @(
    'tool', 'run', 'vpk', '--',
    'pack',
    '--packId', $PackId,
    '--packVersion', $safeVersion,
    '--packDir', $publishDirectory,
    '--mainExe', $MainExe,
    '--outputDir', $releasesDirectory,
    '--channel', $Channel,
    '--runtime', $RuntimeIdentifier,
    '--packTitle', $PackTitle,
    '--packAuthors', $PackAuthors,
    '--icon', $iconPath
)

if (-not [string]::IsNullOrWhiteSpace($ReleaseNotesPath)) {
    $releaseNotesFullPath = Resolve-NormalizedPath $ReleaseNotesPath
    if (-not (Test-Path -LiteralPath $releaseNotesFullPath -PathType Leaf)) {
        throw "Release notes file not found: $releaseNotesFullPath"
    }

    $packArguments += @('--releaseNotes', $releaseNotesFullPath)
}

Invoke-RepoDotnet -RepoRoot $repoRoot -Arguments $packArguments
Rename-SetupPackage -ReleasesDirectory $releasesDirectory -SetupFileName $SetupFileName

if ($Publish -or $UploadDraft) {
    Ensure-VpkTokenForUpload

    $publishValue = if ($Publish) { 'true' } else { 'false' }
    $releaseName = "Vesper Launcher $safeVersion"
    Write-Host "Uploading Velopack release to $RepoUrl (publish=$publishValue)..."
    Invoke-RepoDotnet -RepoRoot $repoRoot -Arguments @(
        'tool', 'run', 'vpk', '--',
        'upload', 'github',
        '--outputDir', $releasesDirectory,
        '--channel', $Channel,
        '--repoUrl', $RepoUrl,
        '--publish', $publishValue,
        '--releaseName', $releaseName
    )
}
else {
    Write-Host ''
    Write-Host 'Velopack packages were created locally only.'
    Write-Host 'Nothing was uploaded, so users will not receive this update yet.'
    Write-Host 'Use -UploadDraft for a GitHub draft or -Publish when you explicitly want users to update.'
}

Write-Host ''
Write-Host "Release output: $releasesDirectory"
