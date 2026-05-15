#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  Time Tracker — Unified Linux Setup Script                             ║
# ║  Install or uninstall with a single command:                           ║
# ║                                                                        ║
# ║  Install:                                                              ║
# ║    curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup-linux.sh | sudo bash
# ║                                                                        ║
# ║  Uninstall:                                                            ║
# ║    curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup-linux.sh | sudo bash -s -- --uninstall
# ╚══════════════════════════════════════════════════════════════════════════╝

set -euo pipefail

RELEASE_URL="https://github.com/vivek-pk/time-tracker/releases/latest/download"

# ── Root check ────────────────────────────────────────────────────────────────
if [[ "$(id -u)" -ne 0 ]]; then
    echo "This script must be run as root (use sudo)." >&2
    exit 1
fi

# ── Uninstall Flow ────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--uninstall" ]]; then
    echo "Uninstalling Time Tracker..."
    
    if [[ -f "/usr/local/bin/time-tracker" ]]; then
        # Try to use the proper uninstall script if we have it locally, otherwise fetch from github
        TMP_DIR=$(mktemp -d)
        curl -fsSL -o "$TMP_DIR/uninstall-linux.sh" "https://raw.githubusercontent.com/vivek-pk/time-tracker/main/scripts/uninstall-linux.sh" || true
        
        if [[ -f "$TMP_DIR/uninstall-linux.sh" ]]; then
            # We don't want prompts for the silent uninstaller, so we export a variable to bypass
            # but currently uninstall-linux.sh prompts. For a single command uninstall, we just call it.
            # To prevent hanging, we can auto-answer 'y' or 'n'.
            printf "y\ny\n" | bash "$TMP_DIR/uninstall-linux.sh"
        else
            # Manual fallback
            systemctl stop time-tracker 2>/dev/null || true
            systemctl disable time-tracker 2>/dev/null || true
            rm -f /etc/systemd/system/time-tracker.service
            systemctl daemon-reload
            rm -f /usr/local/bin/time-tracker
            rm -f /usr/local/bin/time-tracker-location
        fi
        rm -rf "$TMP_DIR"
    else
        echo "Time Tracker is not installed."
    fi
    exit 0
fi

# ── Install Flow ──────────────────────────────────────────────────────────────
echo "Installing Time Tracker..."

# Detect Architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) FILE_ARCH="amd64" ;;
    aarch64|arm64) FILE_ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

TAR_NAME="time-tracker-linux-${FILE_ARCH}.tar.gz"
TAR_URL="${RELEASE_URL}/${TAR_NAME}"

# Create Temp Directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading $TAR_NAME..."
curl -fsSL -o "$TMP_DIR/$TAR_NAME" "$TAR_URL"

echo "Extracting..."
tar -xzf "$TMP_DIR/$TAR_NAME" -C "$TMP_DIR"

BIN_SRC="$TMP_DIR/time-tracker-linux-${FILE_ARCH}"
LOC_SRC="$TMP_DIR/time-tracker-location-linux-${FILE_ARCH}"
INSTALL_SCRIPT="$TMP_DIR/scripts/install-linux.sh"

if [[ ! -f "$INSTALL_SCRIPT" ]]; then
    echo "Failed to find install script inside the archive." >&2
    exit 1
fi

echo "Running installer..."
bash "$INSTALL_SCRIPT" "$BIN_SRC" "$LOC_SRC"

echo "Installation complete! Service is running."
echo "Use 'systemctl status time-tracker' to check."
