#!/usr/bin/env bash
# install-linux.sh — Install time-tracker as a systemd service on Linux.
#
# Must be run as root (or with sudo).
# Usage: sudo bash scripts/install-linux.sh [path/to/binary]

set -euo pipefail

# ── constants ─────────────────────────────────────────────────────────────────
BINARY_SRC="${1:-./bin/time-tracker}"
BINARY_DST="/usr/local/bin/time-tracker"
LOC_SRC="${2:-./bin/time-tracker-location}"
LOC_DST="/usr/local/bin/time-tracker-location"
SERVICE_SRC="./systemd/time-tracker.service"
SERVICE_DST="/etc/systemd/system/time-tracker.service"
ENV_SRC="./.env.example"
ENV_DST="/etc/time-tracker/.env"
DB_DIR="/var/lib/time-tracker"
LOG_DIR="/var/log/time-tracker"
CONF_DIR="/etc/time-tracker"

# ── helpers ───────────────────────────────────────────────────────────────────
info()  { echo "  [+] $*"; }
warn()  { echo "  [!] $*" >&2; }
fatal() { echo "  [✗] $*" >&2; exit 1; }

# ── root check ────────────────────────────────────────────────────────────────
if [[ "$(id -u)" -ne 0 ]]; then
    fatal "This script must be run as root.  Use: sudo bash scripts/install-linux.sh"
fi

# ── sanity checks ─────────────────────────────────────────────────────────────
[[ -f "$BINARY_SRC" ]] || fatal "Binary not found at $BINARY_SRC — run 'make build' first"
[[ -f "$SERVICE_SRC" ]] || fatal "Service unit not found at $SERVICE_SRC"

# ── stop existing service if running ──────────────────────────────────────────
if systemctl is-active --quiet time-tracker 2>/dev/null; then
    info "Stopping existing time-tracker service"
    systemctl stop time-tracker
fi

# ── create directories ────────────────────────────────────────────────────────
info "Creating directories"
for dir in "$DB_DIR" "$LOG_DIR" "$CONF_DIR"; do
    mkdir -p "$dir"
    chmod 750 "$dir"
done

# ── install binary ────────────────────────────────────────────────────────────
info "Installing binary → $BINARY_DST"
cp "$BINARY_SRC" "$BINARY_DST"
chmod 755 "$BINARY_DST"

# ── install location helper ───────────────────────────────────────────────────
if [[ -f "$LOC_SRC" ]]; then
    info "Installing location helper → $LOC_DST"
    cp "$LOC_SRC" "$LOC_DST"
    chmod 755 "$LOC_DST"
else
    warn "Location helper not found at $LOC_SRC — location tracking will use IP geolocation"
    warn "  To build: make build-linux (includes location helper)"
fi

# ── install config (only if not already present) ──────────────────────────────
if [[ -f "$ENV_DST" ]]; then
    warn "Config $ENV_DST already exists — not overwriting.  Edit it manually."
else
    info "Installing config template → $ENV_DST"
    if [[ -f "$ENV_SRC" ]]; then
        cp "$ENV_SRC" "$ENV_DST"
    else
        cat > "$ENV_DST" << 'ENVFILE'
# /etc/time-tracker/.env  (OPTIONAL OVERRIDE FILE)
# All configuration is embedded in the binary (config.json).
# Uncomment variables below ONLY if you need to override them on this specific machine.

# SYNC_API_URL=https://your-api-endpoint.example.com/attendance
# SYNC_API_KEY=
# MORNING_SYNC_HOUR=6
# EVENING_SYNC_HOUR=18
# EVENING_SYNC_MINUTE=30
# IDLE_THRESHOLD_MINUTES=5
# POLL_INTERVAL_SECONDS=30
# DB_PATH=/var/lib/time-tracker/tracker.db
# LOG_PATH=/var/log/time-tracker
# RETENTION_DAYS=3
# SYNC_TIMEOUT_SECONDS=30
ENVFILE
    fi
    chmod 640 "$ENV_DST"
    warn "IMPORTANT: edit $ENV_DST and set SYNC_API_URL before the service starts"
fi

# ── install systemd unit ─────────────────────────────────────────────────────
info "Installing systemd unit → $SERVICE_DST"
cp "$SERVICE_SRC" "$SERVICE_DST"
chmod 644 "$SERVICE_DST"

# ── enable and start service ─────────────────────────────────────────────────
info "Enabling and starting service"
systemctl daemon-reload
systemctl enable time-tracker
systemctl start time-tracker

# Show status
sleep 2
if systemctl is-active --quiet time-tracker; then
    info "Service is running"
else
    warn "Service may not have started.  Check: systemctl status time-tracker"
    warn "Logs: journalctl -u time-tracker -f"
fi

echo ""
echo "Installation complete."
echo "  Config file : $ENV_DST"
echo "  Logs        : journalctl -u time-tracker -f"
echo "  Database    : $DB_DIR/tracker.db"
echo ""
echo "Edit $ENV_DST with the correct SYNC_API_URL,"
echo "then reload: sudo systemctl restart time-tracker"
