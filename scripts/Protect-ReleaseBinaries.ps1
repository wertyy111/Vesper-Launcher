[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$PublishDirectory,

    [string]$AssemblyName = 'VesperLauncher.dll'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$publishRoot = if ([System.IO.Path]::IsPathRooted($PublishDirectory)) {
    [System.IO.Path]::GetFullPath($PublishDirectory)
}
else {
    [System.IO.Path]::GetFullPath((Join-Path $repoRoot $PublishDirectory))
}

if (-not (Test-Path -LiteralPath $publishRoot -PathType Container)) {
    throw "Publish directory was not found: $publishRoot"
}

$assemblyPath = Join-Path $publishRoot $AssemblyName
if (-not (Test-Path -LiteralPath $assemblyPath -PathType Leaf)) {
    throw "Launcher assembly was not found: $assemblyPath"
}

Get-ChildItem -LiteralPath $publishRoot -Recurse -File |
    Where-Object { $_.Extension -in @('.pdb', '.map', '.cs', '.csproj', '.sln') } |
    Remove-Item -Force

$workRoot = Join-Path $publishRoot '.release-protection'
$outputRoot = Join-Path $workRoot 'output'
$toolRoot = Join-Path $workRoot 'tools'
$configPath = Join-Path $workRoot 'obfuscar.xml'
New-Item -ItemType Directory -Force -Path $outputRoot, $toolRoot | Out-Null

$escapedInput = [System.Security.SecurityElement]::Escape($publishRoot)
$escapedOutput = [System.Security.SecurityElement]::Escape($outputRoot)
$escapedAssembly = [System.Security.SecurityElement]::Escape($assemblyPath)
$config = @"
<?xml version="1.0" encoding="utf-8"?>
<Obfuscator>
  <Var name="InPath" value="$escapedInput" />
  <Var name="OutPath" value="$escapedOutput" />
  <Var name="KeepPublicApi" value="false" />
  <Var name="HidePrivateApi" value="true" />
  <Var name="HideStrings" value="true" />
  <Var name="SuppressIldasm" value="true" />
  <Var name="RegenerateDebugInfo" value="false" />
  <Var name="UseUnicodeNames" value="true" />
  <Var name="RenameProperties" value="false" />
  <Var name="RenameEvents" value="false" />
  <Var name="SkipSpecialName" value="true" />
  <Var name="SkipGenerated" value="true" />
  <Var name="ReuseNames" value="true" />
  <Module file="$escapedAssembly">
    <SkipType name="*" decorator="System.Runtime.CompilerServices.CompilerGeneratedAttribute" skipMethods="true" skipFields="true" skipProperties="true" skipEvents="true" />
    <SkipMethod type="*" name=".ctor" />
    <SkipMethod type="*" name=".cctor" />
  </Module>
</Obfuscator>
"@
[System.IO.File]::WriteAllText($configPath, $config, [System.Text.UTF8Encoding]::new($false))

dotnet tool install Obfuscar.GlobalTool --version 2.2.50 --tool-path $toolRoot
if ($LASTEXITCODE -ne 0) {
    throw "Obfuscar tool installation failed with exit code $LASTEXITCODE."
}

$obfuscar = Get-ChildItem -LiteralPath $toolRoot -File |
    Where-Object { $_.Name -match '^obfuscar\.console(?:\.exe)?$' } |
    Select-Object -First 1
if ($null -eq $obfuscar) {
    throw "Obfuscar executable was not found in $toolRoot."
}

& $obfuscar.FullName $configPath
if ($LASTEXITCODE -ne 0) {
    throw "Obfuscar failed with exit code $LASTEXITCODE."
}

$protectedAssembly = Join-Path $outputRoot $AssemblyName
if (-not (Test-Path -LiteralPath $protectedAssembly -PathType Leaf)) {
    throw "Obfuscar output was not found: $protectedAssembly"
}

Copy-Item -LiteralPath $protectedAssembly -Destination $assemblyPath -Force
Remove-Item -LiteralPath $workRoot -Recurse -Force

$forbiddenReleaseFiles = Get-ChildItem -LiteralPath $publishRoot -Recurse -File |
    Where-Object { $_.Extension -in @('.pdb', '.map', '.cs', '.csproj', '.sln', '.user', '.suo') }
if ($forbiddenReleaseFiles) {
    throw "Release contains source or debug files: $($forbiddenReleaseFiles.FullName -join ', ')"
}

$credentialPatterns = @(
    'ghp_[A-Za-z0-9]{20,}',
    'github_pat_[A-Za-z0-9_]{20,}',
    'cfat_[A-Za-z0-9_-]{20,}',
    'Bearer\s+B64:[A-Za-z0-9+/=]{20,}',
    '-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'
)
$textExtensions = @('.json', '.js', '.html', '.css', '.xml', '.config', '.txt', '.yml', '.yaml', '.ps1', '.sh')
foreach ($file in Get-ChildItem -LiteralPath $publishRoot -Recurse -File) {
    if ($file.Extension -notin $textExtensions) {
        continue
    }

    $content = [System.IO.File]::ReadAllText($file.FullName)
    foreach ($pattern in $credentialPatterns) {
        if ($content -match $pattern) {
            throw "Release credential guard rejected: $($file.FullName)"
        }
    }
}

Write-Host "Protected launcher assembly: $assemblyPath"
