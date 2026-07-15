# Cross-platform release CI

Workflow: `.github/workflows/cross-platform-release.yml`.

It builds:

- Linux assets on `ubuntu-latest`: AppImage, `.deb`, `.nupkg`, and release indexes.
- Intel macOS assets on `macos-15-intel` and Apple Silicon assets on `macos-15`: `.dmg`, portable `.zip`, Velopack macOS packages, `.nupkg`, and release indexes.
- Windows assets on `windows-latest`: setup `.exe`, `.nupkg`, and release indexes.

Local packaging:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\Build-Installers.ps1
```

Local output is written to `_dist/installers/<version>/`:

- `windows/VesperLauncherSetup.exe`
- `windows/Vesper.Launcher-win-Setup.exe`
- `windows/Vesper.Launcher-<version>-full.nupkg`
- `linux-x64/VesperLauncher-<version>-linux-x64.tar.gz`
- `linux-x64/install-linux.sh`
- `linux-x64/vesper-launcher_<version>_amd64.deb`
- `osx-x64/VesperLauncher-<version>-osx-x64-app.tar.gz`
- `osx-x64/install-macos.command`
- `osx-arm64/VesperLauncher-<version>-osx-arm64-app.tar.gz`
- `osx-arm64/install-macos.command`

Use GitHub Actions for Linux AppImage and native macOS DMG/pkg. Windows and Linux `.deb` can be packaged locally.

Manual run:

1. Open GitHub Actions.
2. Run `Cross-platform release`.
3. Set `version` to a semantic version without `v`, for example `1.0.60-beta.1`.
4. Set the matching tag, for example `v1.0.60-beta.1`.
5. Keep `upload_release=true` to upload assets to the GitHub Release.

Tag run:

- Push a semantic-version tag matching `v*`.
- The workflow uploads assets to the release with the same tag.

Published download names are stable and are validated before the release is created:

- `VesperLauncherSetup.exe`
- `VesperLauncher-linux-x64.AppImage`
- `VesperLauncher-linux-x64.deb`
- `VesperLauncher-osx-arm64.dmg`
- `VesperLauncher-osx-x64.dmg`
- portable macOS ZIP files for both architectures
- `SHA256SUMS.txt` containing checksums for every published artifact

The workflow performs native installer tests before release:

- runs the raw Linux binary, extracts and runs the AppImage, installs the `.deb`, runs the installed app, and removes the package;
- runs each macOS architecture on matching native hardware, mounts each DMG, and runs the app from the mounted image;
- silently installs the public Windows wrapper, runs the installed obfuscated launcher self-test, verifies Authenticode signatures when configured, and silently uninstalls it.

If this workflow lives outside `wertyy111/Updates-Vesper`, configure `VESPER_RELEASE_TOKEN`
with Contents write access to that repository. The regular `GITHUB_TOKEN` is sufficient when
the workflow runs in the release repository itself.

macOS signing secrets:

- `MACOS_CERTIFICATE_P12_BASE64`: base64-encoded Developer ID Application `.p12`.
- `MACOS_CERTIFICATE_PASSWORD`: password for the `.p12`.
- `MACOS_CODESIGN_IDENTITY`: exact Developer ID Application identity name.
- `MACOS_INSTALLER_IDENTITY`: exact Developer ID Installer identity name.
- `MACOS_KEYCHAIN_PASSWORD`: optional temporary CI keychain password.

macOS notarization secrets:

- `MACOS_NOTARY_KEY_BASE64`: base64-encoded App Store Connect API `.p8` key.
- `MACOS_NOTARY_KEY_ID`: App Store Connect API key id.
- `MACOS_NOTARY_ISSUER_ID`: App Store Connect issuer id.

Windows signing secrets:

- `WINDOWS_CERTIFICATE_PFX_BASE64`: base64-encoded Authenticode `.pfx`.
- `WINDOWS_CERTIFICATE_PASSWORD`: password for the `.pfx`.

Build-only CI runs can produce unsigned packages for testing. A tag or a manual run with `upload_release=true` fails closed if the Windows signing credentials or the complete macOS signing/notarization credentials are missing.

Unsigned macOS builds are suitable for testing only. Public builds should be signed and notarized
so Gatekeeper can verify them.
