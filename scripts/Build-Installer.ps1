[CmdletBinding()]
param(
    [string]$ProjectPath = 'windows/installer-src/Installer.csproj',
    [string]$LauncherProjectPath = 'MinecraftCheatLauncher/MinecraftCheatLauncher.csproj',
    [string]$PayloadZipPath = '_windows_build/payload_latest.zip',
    [string]$Configuration = 'Release',
    [string]$RuntimeIdentifier = 'win-x64',
    [string]$OutputRoot = '_temp_build',
    [switch]$SkipPayloadBuild,
    [switch]$UpdateWindowsSetup
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Resolve-NormalizedPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    return [System.IO.Path]::GetFullPath([Environment]::ExpandEnvironmentVariables($Path))
}

$repoRoot = Resolve-NormalizedPath (Join-Path $PSScriptRoot '..')
$projectFullPath = Resolve-NormalizedPath (Join-Path $repoRoot $ProjectPath)
$launcherProjectFullPath = Resolve-NormalizedPath (Join-Path $repoRoot $LauncherProjectPath)
$payloadZipFullPath = Resolve-NormalizedPath (Join-Path $repoRoot $PayloadZipPath)

if (-not (Test-Path -LiteralPath $projectFullPath -PathType Leaf)) {
    throw "Installer project not found: $projectFullPath"
}

if (-not (Test-Path -LiteralPath $launcherProjectFullPath -PathType Leaf)) {
    throw "Launcher project not found: $launcherProjectFullPath"
}

[xml]$projectXml = Get-Content -LiteralPath $projectFullPath
$version = $projectXml.Project.PropertyGroup.Version | Select-Object -First 1
if ([string]::IsNullOrWhiteSpace($version)) {
    throw "Version is missing in $projectFullPath"
}

$outputRootFullPath = Resolve-NormalizedPath (Join-Path $repoRoot $OutputRoot)
$releaseRoot = Join-Path $outputRootFullPath "release_$version"
$payloadPublishDirectory = Join-Path $releaseRoot 'launcher-payload-self-contained'
$publishDirectory = Join-Path $releaseRoot 'installer-publish-singlefile'
$setupFileName = "VesperLauncherSetup-$version.exe"
$releaseSetupPath = Join-Path $releaseRoot $setupFileName

New-Item -ItemType Directory -Path $releaseRoot -Force | Out-Null

if (-not $SkipPayloadBuild) {
    if (Test-Path -LiteralPath $payloadPublishDirectory) {
        $resolvedPayloadPublishDirectory = Resolve-NormalizedPath $payloadPublishDirectory
        $resolvedReleaseRoot = Resolve-NormalizedPath $releaseRoot
        if (-not $resolvedPayloadPublishDirectory.StartsWith($resolvedReleaseRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to clean unexpected payload publish directory: $resolvedPayloadPublishDirectory"
        }

        Remove-Item -LiteralPath $resolvedPayloadPublishDirectory -Recurse -Force
    }

    New-Item -ItemType Directory -Path $payloadPublishDirectory -Force | Out-Null

    Write-Host "Publishing launcher payload $version with bundled .NET runtime..."
    & dotnet publish $launcherProjectFullPath `
        -c $Configuration `
        -r $RuntimeIdentifier `
        --self-contained true `
        -o $payloadPublishDirectory `
        -nologo `
        -p:PublishSingleFile=false `
        -p:DebugType=none `
        -p:DebugSymbols=false `
        -p:SkipLockedLauncherCheck=true

    if ($LASTEXITCODE -ne 0) {
        throw "launcher dotnet publish failed with exit code $LASTEXITCODE."
    }

    $payloadScriptPath = Join-Path $PSScriptRoot 'New-PayloadZip.ps1'
    if (-not (Test-Path -LiteralPath $payloadScriptPath -PathType Leaf)) {
        throw "Payload zip script not found: $payloadScriptPath"
    }

    Write-Host "Creating self-contained payload zip..."
    & $payloadScriptPath `
        -SourceDirectory $payloadPublishDirectory `
        -DestinationZip $payloadZipFullPath | Write-Host

    if ($LASTEXITCODE -ne 0) {
        throw "payload zip creation failed with exit code $LASTEXITCODE."
    }
}

if (-not (Test-Path -LiteralPath $payloadZipFullPath -PathType Leaf)) {
    throw "Installer payload not found: $payloadZipFullPath"
}

Add-Type -AssemblyName System.IO.Compression.FileSystem
$payloadZip = [System.IO.Compression.ZipFile]::OpenRead($payloadZipFullPath)
try {
    $payloadEntryNames = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    foreach ($entry in $payloadZip.Entries) {
        if (-not [string]::IsNullOrWhiteSpace($entry.Name)) {
            [void]$payloadEntryNames.Add($entry.Name)
        }
    }

    foreach ($runtimeFileName in @('coreclr.dll', 'hostfxr.dll', 'hostpolicy.dll')) {
        if (-not $payloadEntryNames.Contains($runtimeFileName)) {
            throw "Payload does not include bundled .NET runtime file '$runtimeFileName'. Build without -SkipPayloadBuild or use a self-contained launcher publish."
        }
    }
}
finally {
    $payloadZip.Dispose()
}

Write-Host "Publishing installer $version as a single-file executable..."
& dotnet publish $projectFullPath `
    -c $Configuration `
    -r $RuntimeIdentifier `
    --self-contained true `
    -o $publishDirectory `
    -nologo `
    -p:PayloadZip="$payloadZipFullPath" `
    -p:PublishSingleFile=true `
    -p:IncludeNativeLibrariesForSelfExtract=true `
    -p:EnableCompressionInSingleFile=true

if ($LASTEXITCODE -ne 0) {
    throw "dotnet publish failed with exit code $LASTEXITCODE."
}

$publishedSetupPath = Join-Path $publishDirectory 'VesperLauncherSetup.exe'
if (-not (Test-Path -LiteralPath $publishedSetupPath -PathType Leaf)) {
    throw "Published installer not found: $publishedSetupPath"
}

Copy-Item -LiteralPath $publishedSetupPath -Destination $releaseSetupPath -Force

if ($UpdateWindowsSetup) {
    $windowsSetupPath = Join-Path $repoRoot 'windows/setup.exe'
    Copy-Item -LiteralPath $publishedSetupPath -Destination $windowsSetupPath -Force
    Write-Host "Updated: $windowsSetupPath"
}

$releaseSetupItem = Get-Item -LiteralPath $releaseSetupPath
Write-Host ''
Write-Host "Installer ready: $releaseSetupPath"
Write-Host ("Size: {0:N0} bytes" -f $releaseSetupItem.Length)
Write-Host ''
