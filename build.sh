#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd "$DIR"

echo "=== Building Vesper Launcher (Go + Wails v2) ==="

# Add Go / Wails to PATH if not already present
export PATH="$HOME/go/bin:$HOME/.local/go/bin:$PATH"

if ! command -v wails &>/dev/null; then
    echo "Installing/verifying Wails CLI v2..."
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
fi

# Build for current platform
wails build -tags "webkit2_41" "$@"

echo ""
echo "=== Build Successful! ==="
echo "Binary location: $DIR/build/bin/VesperLauncher"
echo "To run launcher:"
echo "  $DIR/build/bin/VesperLauncher"
