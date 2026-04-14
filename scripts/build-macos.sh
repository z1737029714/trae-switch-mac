#!/bin/zsh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
OUTPUT_BIN="$ROOT_DIR/build/bin/trae-switch"
CLI_CONFIG="$ROOT_DIR/build/bin/config.json"
APP_BUNDLE="$ROOT_DIR/build/Trae Switch.app"
APP_CONTENTS_DIR="$APP_BUNDLE/Contents"
APP_MACOS_DIR="$APP_CONTENTS_DIR/MacOS"
APP_RESOURCES_DIR="$APP_CONTENTS_DIR/Resources"
APP_BIN="$APP_MACOS_DIR/trae-switch"
APP_CONFIG="$APP_MACOS_DIR/config.json"
APP_PLIST="$APP_CONTENTS_DIR/Info.plist"
APP_ICON_SOURCE="$ROOT_DIR/build/appicon.png"
APP_ICON_NAME="AppIcon"
APP_CUSTOM_ICON_FILE="$APP_BUNDLE/Icon"$'\r'

export GOCACHE="${GOCACHE:-/tmp/trae-switch-go-build}"
export GOMODCACHE="${GOMODCACHE:-/tmp/trae-switch-go-mod}"
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"

PROXY_HOST="${TRAE_SWITCH_PROXY_HOST:-127.0.0.1}"
PROXY_PORT="${TRAE_SWITCH_PROXY_PORT:-7890}"

if command -v nc >/dev/null 2>&1 && nc -z "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1; then
  export HTTPS_PROXY="http://${PROXY_HOST}:${PROXY_PORT}"
  export HTTP_PROXY="http://${PROXY_HOST}:${PROXY_PORT}"
  export ALL_PROXY="socks5://${PROXY_HOST}:${PROXY_PORT}"
fi

mkdir -p "$ROOT_DIR/build/bin"
mkdir -p "$APP_MACOS_DIR" "$APP_RESOURCES_DIR"

if [[ ! -d "$FRONTEND_DIR/node_modules" ]]; then
  echo "Installing frontend dependencies..."
  (cd "$FRONTEND_DIR" && npm install)
fi

echo "Building frontend..."
(cd "$FRONTEND_DIR" && npm run build)

echo "Building macOS app binary..."
CGO_LDFLAGS='-framework UniformTypeIdentifiers -mmacosx-version-min=10.13' \
  go build -tags production -o "$OUTPUT_BIN" "$ROOT_DIR"

cp "$OUTPUT_BIN" "$APP_BIN"
chmod +x "$APP_BIN"

if [[ -f "$APP_ICON_SOURCE" ]]; then
  ICON_RSRC="$(mktemp /tmp/trae-switch.icon.rsrc.XXXXXX)"
  trap 'rm -f "$ICON_RSRC"' EXIT

  rm -f "$APP_CUSTOM_ICON_FILE"
  cp "$APP_ICON_SOURCE" "$APP_RESOURCES_DIR/${APP_ICON_NAME}.png"
  sips -i "$APP_ICON_SOURCE" >/dev/null
  xcrun DeRez -only icns "$APP_ICON_SOURCE" > "$ICON_RSRC"
  xcrun Rez -append "$ICON_RSRC" -o "$APP_CUSTOM_ICON_FILE"
  xcrun SetFile -a C "$APP_BUNDLE"
  xcrun SetFile -a V "$APP_CUSTOM_ICON_FILE"
fi

cat > "$APP_PLIST" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleDisplayName</key>
  <string>Trae Switch</string>
  <key>CFBundleExecutable</key>
  <string>trae-switch</string>
  <key>CFBundleIdentifier</key>
  <string>com.z1737029714.trae-switch</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Trae Switch</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0.0</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSMinimumSystemVersion</key>
  <string>11.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
EOF

if [[ ! -f "$APP_CONFIG" && -f "$CLI_CONFIG" ]]; then
  cp "$CLI_CONFIG" "$APP_CONFIG"
fi

echo "Build finished: $APP_BUNDLE"
