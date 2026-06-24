#!/usr/bin/env bash
set -euo pipefail

publish_dir="${1:?publish directory is required}"
output_dir="${2:?output directory is required}"
version="${3:-beta-1.0}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
appdir="${output_dir}/VesperLauncher.AppDir"
appimagetool="${output_dir}/appimagetool-x86_64.AppImage"
output_path="${output_dir}/VesperLauncher-${version}-linux-x64.AppImage"

rm -rf "$appdir"
mkdir -p "$appdir/usr/bin" \
  "$appdir/usr/share/applications" \
  "$appdir/usr/share/icons/hicolor/256x256/apps" \
  "$output_dir"

cp -a "${publish_dir}/." "$appdir/usr/bin/"
chmod +x "$appdir/usr/bin/VesperLauncher"

icon_source="${repo_root}/MinecraftCheatLauncher/Assets/vesper-logo.png"
cp "$icon_source" "$appdir/usr/share/icons/hicolor/256x256/apps/vesper-launcher.png"
cp "$icon_source" "$appdir/vesper-launcher.png"

cat > "$appdir/usr/share/applications/vesper-launcher.desktop" <<'DESKTOP'
[Desktop Entry]
Type=Application
Name=Vesper Launcher
Comment=Minecraft Java launcher
Exec=VesperLauncher
Icon=vesper-launcher
Categories=Game;
Terminal=false
DESKTOP

cp "$appdir/usr/share/applications/vesper-launcher.desktop" "$appdir/vesper-launcher.desktop"

cat > "$appdir/AppRun" <<'APPRUN'
#!/usr/bin/env bash
set -euo pipefail
here="$(dirname "$(readlink -f "$0")")"
exec "$here/usr/bin/VesperLauncher" "$@"
APPRUN
chmod +x "$appdir/AppRun"

if [[ ! -x "$appimagetool" ]]; then
  curl -L --fail --retry 3 \
    -o "$appimagetool" \
    "https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage"
  chmod +x "$appimagetool"
fi

rm -f "$output_path"
ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 "$appimagetool" "$appdir" "$output_path"
chmod +x "$output_path"

echo "$output_path"
