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

Velopack does not make compiled .NET code impossible to reverse engineer. For stronger protection later, add code signing, trimming where safe, and obfuscation for release builds. Never rely on a secret embedded in the launcher binary.

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

Velopack update packages inside the same folder can still contain versions in their metadata and package names. Do not rename `.nupkg`, `RELEASES`, or `releases.win.json` manually.

## Draft upload

This uploads a GitHub draft release, still not visible to normal users.

```powershell
$env:VPK_TOKEN = "YOUR_FINE_GRAINED_UPDATES_REPO_TOKEN"

.\scripts\Publish-VelopackUpdate.ps1 `
  -UploadDraft `
  -RepoUrl "https://github.com/wertyy111/Updates-Vesper"
```

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

- old releases are deleted;
- old tags are deleted when GitHub allows it;
- the internal Velopack setup asset is removed from the release;
- the public setup asset stays named `VesperLauncherSetup.exe`.

This prevents users from downloading old launcher builds from the updates repository. Velopack package metadata still contains the current package version internally.

## Client config

The launcher reads `MinecraftCheatLauncher/update-config.json`.

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
