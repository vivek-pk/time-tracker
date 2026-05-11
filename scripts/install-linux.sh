#!/usr/bin/env bash
# install-linux.sh - Install time-tracker as a systemd service on Linux.
#
# Must be run as root (or with sudo).
# Usage: sudo bash scripts/install-linux.sh [path/to/binary] [path/to/location-binary]

set -euo pipefail

# -- constants ---------------------------------------------------------------
BINARY_SRC="${1:-./bin/time-tracker}"
LOC_SRC="${2:-./bin/time-tracker-location}"
BINARY_DST="/usr/local/bin/time-tracker"
LOC_DST="/usr/local/bin/time-tracker-location"
SERVICE_DST="/etc/systemd/system/time-tracker.service"
DB_DIR="/var/lib/time-tracker"
LOG_DIR="/var/log/time-tracker"

# Resolve the script's own directory so we can find sibling files
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# The systemd unit could be alongside the script or in a parent systemd/ dir
if [[ -f "$SCRIPT_DIR/../systemd/time-tracker.service" ]]; then
    SERVICE_SRC="$SCRIPT_DIR/../systemd/time-tracker.service"
elif [[ -f "$SCRIPT_DIR/time-tracker.service" ]]; then
    SERVICE_SRC="$SCRIPT_DIR/time-tracker.service"
else
    SERVICE_SRC=""
fi

# -- helpers -----------------------------------------------------------------
info()  { echo "  [+] $*"; }
warn()  { echo "  [!] $*" >&2; }
fatal() { echo "  [x] $*" >&2; exit 1; }

# -- root check --------------------------------------------------------------
if [[ "$(id -u)" -ne 0 ]]; then
    fatal "This script must be run as root.  Use: sudo bash $0"
fi

# -- sanity checks -----------------------------------------------------------
[[ -f "$BINARY_SRC" ]] || fatal "Binary not found at $BINARY_SRC"

if [[ -z "$SERVICE_SRC" ]]; then
    # Generate the systemd unit inline if not found in the archive
    info "Generating systemd service unit"
    SERVICE_SRC="/tmp/time-tracker.service"
    cat > "$SERVICE_SRC" << 'UNIT'
[Unit]
Description=Time Tracker - Activity Monitor
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/time-tracker
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT
fi

# -- stop existing service if running ----------------------------------------
if systemctl is-active --quiet time-tracker 2>/dev/null; then
    info "Stopping existing time-tracker service"
    systemctl stop time-tracker
fi

# -- create directories ------------------------------------------------------
info "Creating directories"
for dir in "$DB_DIR" "$LOG_DIR"; do
    mkdir -p "$dir"
    chmod 750 "$dir"
done

# -- install binary ----------------------------------------------------------
info "Installing binary -> $BINARY_DST"
cp "$BINARY_SRC" "$BINARY_DST"
chmod 755 "$BINARY_DST"

# -- install location helper -------------------------------------------------
if [[ -f "$LOC_SRC" ]]; then
    info "Installing location helper -> $LOC_DST"
    cp "$LOC_SRC" "$LOC_DST"
    chmod 755 "$LOC_DST"
else
    warn "Location helper not found - location tracking will use IP geolocation"
fi

# -- install systemd unit ----------------------------------------------------
info "Installing systemd unit -> $SERVICE_DST"
cp "$SERVICE_SRC" "$SERVICE_DST"
chmod 644 "$SERVICE_DST"

# -- enable and start service ------------------------------------------------
info "Enabling and starting service"
systemctl daemon-reload
systemctl enable time-tracker
systemctl start time-tracker

# Show status
sleep 2
if systemctl is-active --quiet time-tracker; then
    info "Service is running"
else
    warn "Service may not have started. Check: systemctl status time-tracker"
fi

# -- Read Machine ID ---------------------------------------------------------
MACHINE_ID=""
if [[ -f /etc/machine-id ]]; then
    MACHINE_ID=$(cat /etc/machine-id)
elif command -v hostname &>/dev/null; then
    MACHINE_ID=$(hostname)
fi

echo ""
echo "==========================================================="
echo "Installation complete."
echo "==========================================================="
echo ""
echo "  Machine ID  : $MACHINE_ID"
echo "  Config      : embedded in binary (config.json at build time)"
echo "  Database    : $DB_DIR/tracker.db"
echo "  Logs        : journalctl -u time-tracker -f"
echo "               : $LOG_DIR/output.log"
echo ""
echo "  ** Copy the Machine ID above to register this machine in your HRMS **"
echo ""
echo "  To check status: systemctl status time-tracker"
echo "  To view logs:    journalctl -u time-tracker -f"
