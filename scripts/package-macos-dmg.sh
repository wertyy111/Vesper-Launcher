#!/usr/bin/env bash
set -euo pipefail

publish_dir="${1:?publish directory is required}"
rid="${2:?rid is required}"
output_dir="${3:?output directory is required}"
version="${4:-beta-1.0}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
staging="${output_dir}/staging-${rid}"
app_bundle="${staging}/Vesper Launcher.app"
contents="${app_bundle}/Contents"
macos_dir="${contents}/MacOS"
resources_dir="${contents}/Resources"
dmg_path="${output_dir}/VesperLauncher-${version}-${rid}.dmg"
codesign_identity="${MACOS_CODESIGN_IDENTITY:-}"

rm -rf "$staging" "$dmg_path"
mkdir -p "$macos_dir" "$resources_dir" "$output_dir"

cp -a "${publish_dir}/." "$macos_dir/"
chmod +x "$macos_dir/VesperLauncher"

icon_source="${repo_root}/MinecraftCheatLauncher/Assets/vesper-logo.png"
iconset="${staging}/VesperLauncher.iconset"
mkdir -p "$iconset"
sips -z 16 16 "$icon_source" --out "$iconset/icon_16x16.png" >/dev/null
sips -z 32 32 "$icon_source" --out "$iconset/icon_16x16@2x.png" >/dev/null
sips -z 32 32 "$icon_source" --out "$iconset/icon_32x32.png" >/dev/null
sips -z 64 64 "$icon_source" --out "$iconset/icon_32x32@2x.png" >/dev/null
sips -z 128 128 "$icon_source" --out "$iconset/icon_128x128.png" >/dev/null
sips -z 256 256 "$icon_source" --out "$iconset/icon_128x128@2x.png" >/dev/null
sips -z 256 256 "$icon_source" --out "$iconset/icon_256x256.png" >/dev/null
sips -z 512 512 "$icon_source" --out "$iconset/icon_256x256@2x.png" >/dev/null
sips -z 512 512 "$icon_source" --out "$iconset/icon_512x512.png" >/dev/null
sips -z 1024 1024 "$icon_source" --out "$iconset/icon_512x512@2x.png" >/dev/null
iconutil -c icns "$iconset" -o "$resources_dir/VesperLauncher.icns"

cat > "$contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleDisplayName</key>
  <string>Vesper Launcher</string>
  <key>CFBundleExecutable</key>
  <string>VesperLauncher</string>
  <key>CFBundleIconFile</key>
  <string>VesperLauncher</string>
  <key>CFBundleIdentifier</key>
  <string>com.vesper.launcher</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Vesper Launcher</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>${version}</string>
  <key>CFBundleVersion</key>
  <string>${version}</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

echo "APPL????" > "$contents/PkgInfo"

if [[ -n "$codesign_identity" && "$codesign_identity" != "-" ]]; then
  codesign --force --deep --options runtime --timestamp --sign "$codesign_identity" "$app_bundle"
fi

hdiutil create \
  -volname "Vesper Launcher" \
  -srcfolder "$app_bundle" \
  -ov \
  -format UDZO \
  "$dmg_path"

if [[ -n "$codesign_identity" && "$codesign_identity" != "-" ]]; then
  codesign --force --timestamp --sign "$codesign_identity" "$dmg_path"
fi

if [[ -n "${MACOS_NOTARY_KEY_PATH:-}" && -n "${MACOS_NOTARY_KEY_ID:-}" && -n "${MACOS_NOTARY_ISSUER_ID:-}" ]]; then
  xcrun notarytool submit "$dmg_path" \
    --key "$MACOS_NOTARY_KEY_PATH" \
    --key-id "$MACOS_NOTARY_KEY_ID" \
    --issuer "$MACOS_NOTARY_ISSUER_ID" \
    --wait
  xcrun stapler staple "$dmg_path"
fi

echo "$dmg_path"
