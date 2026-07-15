# Velopack updates and source safety

Vesper Launcher uses Velopack for launcher updates. The source repository stays private. The updates repository must contain only compiled Velopack artifacts.

## What is safe

- Keep source code in the private `Vesper-Launcher` repository.
- Publish update artifacts to a separate binary-only repository such as `Updates-Vesper`.
- Do not put GitHub tokens, private certificates, `.pfx` passwords, `.pdb` files, or source archives into release assets.
- Use a fine-grained GitHub token scoped only to the updates repository.
- Keep the token in the current PowerShell session as `VPK_TOKEN` or `GITHUB_TOKEN`.

## What Velopack protects

- It installs and updates the app through signed Velopack package metadata.
- It downloads full and delta packages from the selected channel.
- It refuses to update debug/uninstalled runs where `UpdateManager.IsInstalled` is false.

Velopack does not make compiled .NET code impossible to reverse engineer. Release scripts therefore remove symbols and source artifacts, disable source embedding, and obfuscate `VesperLauncher.dll` with Obfuscar before packaging. The packaged binary is then executed with `--self-test` so an obfuscation error cannot silently reach a release. This raises the cost of reverse engineering, but it is not absolute protection. Never rely on a secret embedded in the launcher binary.

`scripts/Protect-ReleaseBinaries.ps1` is enabled automatically for Release packaging. `-SkipProtection` exists only for diagnostics and must not be used for public artifacts.

The self-test also rejects `.cs`, `.csproj`, `.sln`, `.pdb`, and source-map files and checks the release directory for obvious credential markers.

## Local package test

This creates local packages only. Users will not receive an update.

```powershell
.\scripts\Publish-VelopackUpdate.ps1 -SkipPreviousDownload
```

Output goes to:

```text
_build_verify\velopack\<version>\Releases
```

The script also redirects its temporary packaging directory to the same build root so Velopack does not fill the system `%TEMP%` folder on `C:`.

The user-facing installer is always normalized to one stable file name:

```text
VesperLauncherSetup.exe
```

The standard Velopack `Setup.exe` is a one-click installer. It does not show an install-folder picker in the default UI, but it can install to another drive when launched with:

```powershell
.\VesperLauncherSetup.exe --installto "D:\Games\Vesper Launcher"
```

For a branded first-install experience, the release script builds a Vesper setup wrapper from `windows/installer-src`. The wrapper runs as the current user, lets the user choose a folder, and then starts the embedded Velopack setup with `--installto`. It must not request administrator elevation for this per-user installation. Do not replace the Velopack installation layout with a raw file-copy installer unless the launcher updater is changed too, because Velopack updates rely on its installed layout.

To rebuild only the public wrapper around the latest local Velopack setup:

```powershell
.\scripts\Build-Installer.ps1
```

Velopack update packages inside the same folder still contain the real SemVer package version in metadata. Do not edit the `Version` field inside `releases.win.json` manually, because Velopack uses it to compare updates.

For public beta releases, keep GitHub names clean and human-readable. Example:

```powershell
.\scripts\Publish-VelopackUpdate.ps1 `
  -Publish `
  -PreRelease `
  -RepoUrl "https://github.com/wertyy111/Updates-Vesper" `
  -TagName "beta-1.0" `
  -ReleaseName "Vesper Launcher beta 1.0" `
  -PublicPackageFileName "Vesper.Launcher-beta-1.0-full.nupkg"
```

This keeps the public release page on `beta 1.0` naming while preserving the internal Velopack package version for safe update checks.

By default, `Publish-VelopackUpdate.ps1` also replaces the public `VesperLauncherSetup.exe` with the wrapper. The internal `Vesper.Launcher-win-Setup.exe` is uploaded only so Velopack can create the release correctly, then it is removed from the GitHub Release asset list.

## Draft upload

This uploads a GitHub draft release, still not visible to normal users.

```powershell
$env:VPK_TOKEN = "YOUR_FINE_GRAINED_UPDATES_REPO_TOKEN"

.\scripts\Publish-VelopackUpdate.ps1 `
  -UploadDraft `
  -RepoUrl "https://github.com/wertyy111/Updates-Vesper"
```

For GitHub Actions, store the token as the repository secret `VESPER_RELEASE_TOKEN`. Use a fine-grained token limited to `wertyy111/Updates-Vesper` with only Contents write permission. Never paste a token into chat, source files, workflow YAML, command history, or release assets. Revoke an exposed token immediately and replace it with a new one.

Public workflows require code-signing credentials for Windows and both signing plus notarization credentials for macOS. Build-only workflow runs may remain unsigned, but unsigned assets cannot be published by the release job.

## Real update

Run this only when you explicitly want installed users to update.

```powershell
$env:VPK_TOKEN = "YOUR_FINE_GRAINED_UPDATES_REPO_TOKEN"

.\scripts\Publish-VelopackUpdate.ps1 `
  -Publish `
  -RepoUrl "https://github.com/wertyy111/Updates-Vesper"
```

After publishing, the public installer link should stay stable:

```text
https://github.com/wertyy111/Updates-Vesper/releases/latest/download/VesperLauncherSetup.exe
```

By default, `Publish-VelopackUpdate.ps1` keeps only the newest published GitHub Release:

- an existing release with the same tag is deleted before replacement;
- old releases are deleted;
- old tags are deleted when GitHub allows it;
- the internal Velopack setup asset is removed from the release;
- the public setup asset stays named `VesperLauncherSetup.exe`.
- optional public package naming can hide technical package versions from the asset list.

This prevents users from downloading old launcher builds from the updates repository. Velopack package metadata still contains the current package version internally.

## Client config

The launcher reads `VesperLauncher/update-config.json`.

```json
{
  "Enabled": true,
  "CheckOnStartup": true,
  "InstallOnStartup": true,
  "SourceType": "GitHub",
  "SourceUrl": "https://github.com/wertyy111/Updates-Vesper",
  "Channel": "win",
  "IncludePrerelease": false
}
```

`SourceUrl` must not require a client-side secret. If the updates repository is private, the launcher would need a token to read it, and that token could be extracted from the app. Use a public binary-only updates repository or a signed public object-storage feed instead.

