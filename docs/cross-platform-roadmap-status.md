# Cross-Platform Roadmap Status

Date: 2026-06-22

## Summary

The project is not cross-platform yet. The current state is a Windows/WPF legacy launcher with a Photino + React frontend already present. The migration path is to move backend logic into cross-platform services first, then retire WPF.

## Stage 0: Audit

Status: completed as a repo document.

Output:

- `docs/cross-platform-audit.md`

Findings:

- WPF is still central to the launcher.
- `MainWindow.xaml.cs` and `MinecraftLauncherService.cs` are the main monoliths.
- `MinecraftLauncherService.cs` is not clean backend because it still uses WPF bitmap types.
- Windows-only features are spread across launcher, installer, VesperNet service, scripts, and native window code.

## Stage 1: Target Architecture

Status: partially started.

Implemented:

- `Core/Logger.cs`
- `Core/AppDiagnostics.cs`
- `Core/EventManager.cs`
- `Core/DependencyContainer.cs`
- `Utils/PathHelper.cs`
- `Utils/HashHelper.cs`
- `Storage/FileManager.cs`
- `Storage/CacheManager.cs`

Not implemented yet:

- `AppHost`
- `ConfigManager`
- `PhotinoLauncherShell`
- `FrontendBridge`
- `CommandRouter`
- `LauncherStateProvider`
- `LauncherCommandHandler`
- `MinecraftLauncher`
- `JavaDetector`
- `JvmArgBuilder`
- `VersionManager`
- `VersionStateMachine`
- `ModManager`
- `PerformanceOptimizer`
- `VesperHttpClient`
- `DownloadManager`
- `UpdateChecker`
- `ApiService`
- `SkinManager`
- `SkinCache`
- `SkinRenderer`
- `SkinUploader`
- `DatabaseManager`
- `IPlatformService`
- `WindowsPlatformService`
- `LinuxPlatformService`
- `MacOsPlatformService`
- `PlatformProcessService`
- `PlatformPathService`

## Stage 2: Stabilization Without Deepening WPF

Status: partially started.

### TASK-03: Close Game Button

Status: implemented first pass.

Implemented:

- `Launcher/ProcessMonitor.cs`
- `Process.Exited`
- polling fallback every 2 seconds
- graceful close with `CloseMainWindow()`
- forced fallback with `Kill(entireProcessTree: true)`
- `MainWindow` now uses `ProcessMonitor.IsRunning` for game-running state

Needs verification:

- Build check
- Manual launch/close test
- Manual close-from-Minecraft test

### TASK-04: Log Paths

Status: implemented first pass.

Implemented:

- cross-platform path helper
- log session directories under user data
- username/path segment sanitization
- logger rotation
- `Program.cs`, `App.xaml.cs`, and `MainWindow.xaml.cs` now write through `Logger`

Needs verification:

- Confirm actual runtime path on Windows
- Confirm path behavior on Linux/macOS later

### TASK-02: Weak PC Startup

Status: partially implemented.

Implemented:

- startup diagnostics for OS, runtime, .NET, architecture, RAM, working set, elevation, base path
- existing global exception handlers remain in `App.xaml.cs`
- Photino shell logs fatal unhandled exceptions

Not implemented yet:

- user-friendly crash report window/page in React shell
- AppHost-level startup wrapper
- low-memory mode flags
- removal of WPF-only startup behavior

### TASK-01: RAM

Status: not started.

Needed:

- timer ownership audit
- event subscription cleanup audit
- cache TTL implementation
- bitmap/image disposal review
- RAM cache cleanup after game exit

## Stage 3: Minecraft Launch

Status: not started, except reusable `ProcessMonitor`.

Next classes:

- `JvmArgBuilder`
- `JavaDetector`
- `MinecraftLauncher`
- extraction from `MinecraftLauncherService.cs`

Risks:

- `MinecraftLauncherService.cs` mixes launch, downloads, versions, skins, mods, and WPF bitmap work.

## Stage 4: Versions

Status: not started.

Needed:

- `VersionManager`
- `VersionStateMachine`
- install/play/update states
- checksum integrity
- cache-first loading
- background refresh

## Stage 5: Skins

Status: not started.

Needed:

- `SkinCache`
- `SkinRenderer`
- `SkinManager`
- `SkinUploader`

Important correction:

- Do not promise Hypixel support for Vesper-hosted skins. Online-mode public servers use Mojang-signed textures.

## Stage 6: Mods

Status: not started.

Needed:

- `ModManager`
- Modrinth/Forge/Fabric catalog extraction
- catalog cache
- optimization mod recommendations

## Stage 7: Network/API/Download

Status: not started.

Needed:

- `VesperHttpClient`
- `DownloadManager`
- `ApiService`
- `UpdateChecker`

## Stage 8: Platform Layer

Status: not started.

Needed:

- `IPlatformService`
- `IPlatformProcessService`
- `IPlatformPathService`
- Windows/Linux/macOS implementations
- disabled-state handling for Windows-only features

## Stage 9: Remove WPF

Status: not started.

Blocked by:

- React/Photino bridge does not own all workflows yet.
- Backend still depends on WPF types.
- `MainWindow` still owns most launcher behavior.

## Stage 10: Verification

Status: blocked locally.

Reason:

- No .NET SDK is available in the current machine environment. `dotnet --list-sdks` returns empty.

Required later:

- build
- launch launcher
- launch Minecraft
- close Minecraft manually
- load versions
- test skins
- RAM idle/active measurement
- Linux/macOS compatibility checks
