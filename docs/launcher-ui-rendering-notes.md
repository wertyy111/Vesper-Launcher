# Vesper Launcher UI rendering notes

## Ghost clicks / window goes behind

The old bug where bottom/top buttons sent the launcher behind the desktop was caused by invisible native/WPF host hit-test areas sitting above the Photino UI. Keep host windows hidden, non-hit-testable, and never place draggable regions above interactive React controls. In React, button handlers should stop pointer propagation before sending commands to the host.

## Liquid glass ghosting

When the day/night background changes, glass layers can keep a stale captured background. Force a React remount with a theme/background key, then ask `liquidGL` to recapture and render its snapshot. Glass layers should keep `transform: translateZ(0)` and `will-change: transform, backdrop-filter`.

## macOS icon squares

Do not use Segoe MDL2 private-use glyphs for critical controls on macOS/Linux. Use bundled SVG/icon fonts, lucide, or safe text symbols. Private Windows glyphs render as squares on macOS.

## macOS startup updates

The Photino startup splash runs `LauncherAutoUpdateService` before the main UI loads on every platform. Windows uses the `win` channel, macOS uses `osx-arm64` or `osx-x64`, and Linux uses `linux-x64`. Updates are applied only when Velopack recognizes the current installation; portable and raw development builds continue without attempting to modify themselves.
