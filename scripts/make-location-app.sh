#!/bin/sh
# Creates a minimal .app bundle wrapping time-tracker-location for testing.
# Needed because macOS locationd only shows the permission dialog for app
# bundles launched via `open`. The installed LaunchAgent binary gets its own
# permission dialog on first run.
#
# Usage:
#   bash scripts/make-location-app.sh          # dev binary (./bin/)
#   bash scripts/make-location-app.sh /path/to/binary  # any binary
set -e

PROJ="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="${1:-$PROJ/bin/time-tracker-location}"
APP="$PROJ/bin/time-tracker-location.app"
CONTENTS="$APP/Contents"
MACOS="$CONTENTS/MacOS"

if [ ! -f "$BINARY" ]; then
  echo "Build first: make build-location"
  exit 1
fi

mkdir -p "$MACOS"
# Copy UNSIGNED binary into bundle, let `codesign` on the bundle sign it.
# Then copy the signed binary BACK so standalone and bundle share the same
# cdhash — meaning a single permission grant via `open -W app` covers both.
cp -f "$BINARY" "$MACOS/time-tracker-location"

# (Info.plist is written next, then we sign the whole bundle)

cat > "$CONTENTS/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>com.timetracker.locationhelper</string>
    <key>CFBundleName</key>
    <string>time-tracker-location</string>
    <key>CFBundleExecutable</key>
    <string>time-tracker-location</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSLocationWhenInUseUsageDescription</key>
    <string>Time Tracker records your location at the start of each work session for attendance purposes.</string>
    <key>NSLocationAlwaysAndWhenInUseUsageDescription</key>
    <string>Time Tracker records your location at the start of each work session for attendance purposes.</string>
</dict>
</plist>
PLIST

# Sign the whole bundle — this signs the inner binary and the bundle container.
codesign -f -s - --entitlements "$PROJ/entitlements/location-helper.plist" "$APP"

# Copy the now-signed binary BACK to the standalone path so both share the same cdhash.
cp -f "$MACOS/time-tracker-location" "$BINARY"

echo "App bundle: $APP"
echo ""
echo "Run once to trigger the location permission dialog:"
echo "  open -W \"$APP\""
echo ""
echo "After granting, the binary works standalone:"
echo "  $BINARY"

