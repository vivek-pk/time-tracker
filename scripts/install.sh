#!/usr/bin/env bash
# install.sh — Install time-tracker as a macOS system daemon.
#
# Must be run as root (or with sudo).  The Makefile target `make install`
# calls this script automatically after building the binary.
#
# What this script does
# ─────────────────────
# 1. Verifies it is running as root.
# 2. Creates the required directories with correct ownership.
# 3. Copies the binary to /usr/local/bin.
# 4. Installs the .env config template to /etc/time-tracker/.env
#    (only if one does not already exist, to preserve operator edits).
# 5. Copies the launchd plist to /Library/LaunchDaemons.
# 6. Sets strict permissions on the plist (root:wheel 0644).
# 7. Loads (or reloads) the daemon.

set -euo pipefail

# ── constants ─────────────────────────────────────────────────────────────────
BINARY_SRC="${1:-./bin/time-tracker}"
BINARY_DST="/usr/local/bin/time-tracker"
LOC_SRC="${2:-./bin/time-tracker-location}"
LOC_DST="/usr/local/bin/time-tracker-location"
PLIST_SRC="./launchd/com.timetracker.daemon.plist"
PLIST_DST="/Library/LaunchDaemons/com.timetracker.daemon.plist"
PLIST_LABEL="com.timetracker.daemon"
AGENT_SRC="./launchd/com.timetracker.locationhelper.plist"
AGENT_DST="/Library/LaunchAgents/com.timetracker.locationhelper.plist"
AGENT_LABEL="com.timetracker.locationhelper"
ENTITLEMENTS="./entitlements/location-helper.plist"
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
    fatal "This script must be run as root.  Use: sudo make install"
fi

# ── sanity checks ─────────────────────────────────────────────────────────────
[[ -f "$BINARY_SRC" ]] || fatal "Binary not found at $BINARY_SRC — run 'make build' first"
[[ -f "$PLIST_SRC"  ]] || fatal "Plist not found at $PLIST_SRC"
[[ -f "$LOC_SRC"    ]] || fatal "Location helper not found at $LOC_SRC — run 'make build-location' first"

# ── create directories ────────────────────────────────────────────────────────
info "Creating directories"
for dir in "$DB_DIR" "$LOG_DIR" "$CONF_DIR"; do
    mkdir -p "$dir"
    chown root:wheel "$dir"
    chmod 750 "$dir"
done

# ── install binary ────────────────────────────────────────────────────────────
info "Installing binary → $BINARY_DST"
cp "$BINARY_SRC" "$BINARY_DST"
chown root:wheel "$BINARY_DST"
chmod 755 "$BINARY_DST"

# ── install location helper binary ───────────────────────────────────────
info "Installing location helper → $LOC_DST"
cp "$LOC_SRC" "$LOC_DST"
chown root:wheel "$LOC_DST"
chmod 755 "$LOC_DST"
# NOTE: The binary must arrive pre-signed (run `bash scripts/make-location-app.sh`
# before `sudo make install`). Re-signing here would change the cdhash and
# invalidate any prior location permission grant.

# ── install config (only if not already present) ──────────────────────────────
if [[ -f "$ENV_DST" ]]; then
    warn "Config $ENV_DST already exists — not overwriting.  Edit it manually."
else
    info "Installing config template → $ENV_DST"
    cp "$ENV_SRC" "$ENV_DST"
    chown root:wheel "$ENV_DST"
    chmod 640 "$ENV_DST"   # root can read/write; wheel can read; others: nothing
    warn "IMPORTANT: edit $ENV_DST and set SYNC_API_URL before the daemon starts"
fi

# ── install plist ─────────────────────────────────────────────────────────────
info "Installing daemon plist → $PLIST_DST"
cp "$PLIST_SRC" "$PLIST_DST"
chown root:wheel "$PLIST_DST"
chmod 644 "$PLIST_DST"   # launchd requires 644 for system daemons

# ── install location agent plist ──────────────────────────────────────────
info "Installing location agent plist → $AGENT_DST"
cp "$AGENT_SRC" "$AGENT_DST"
chown root:wheel "$AGENT_DST"
chmod 644 "$AGENT_DST"

# ── load / reload daemon ──────────────────────────────────────────────────────
# Unload first (ignoring errors if it is not currently loaded).
launchctl unload "$PLIST_DST" 2>/dev/null || true

info "Loading daemon"
launchctl load -w "$PLIST_DST"

# Load the location agent (runs in the user's session, not as root).
CURRENT_USER=$(stat -f '%Su' /dev/console)
if [[ -n "$CURRENT_USER" && "$CURRENT_USER" != "root" ]]; then
    CURRENT_UID=$(id -u "$CURRENT_USER")
    info "Loading location agent for user $CURRENT_USER (uid=$CURRENT_UID)"
    launchctl bootout gui/"$CURRENT_UID"/"$AGENT_LABEL" 2>/dev/null || true
    launchctl bootstrap gui/"$CURRENT_UID" "$AGENT_DST" \
        && info "Location agent loaded" \
        || warn "Could not load location agent automatically — log out and back in"
else
    warn "Could not determine current user — location agent will load on next login"
fi

# Give it a moment to start.
sleep 2

# Show status.
if launchctl list | grep -q "$PLIST_LABEL"; then
    info "Daemon is running (label: $PLIST_LABEL)"
else
    warn "Daemon may not have started.  Check: sudo launchctl list | grep timetracker"
    warn "Logs: tail -f $LOG_DIR/error.log"
fi

echo ""
echo "Installation complete."
echo "  Config file : $ENV_DST"
echo "  Logs        : $LOG_DIR/"
echo "  Database    : $DB_DIR/tracker.db"
echo ""
echo "Edit $ENV_DST with the correct SYNC_API_URL,"
echo "then reload the daemon with: sudo launchctl kickstart -k system/$PLIST_LABEL"
echo ""
echo "FIRST-RUN — Location permission:"
echo "  The location helper needs Location Services access."
echo "  On first run it will ask for permission.  If the dialog does not appear,"
echo "  grant it manually in:"
echo "    System Settings > Privacy & Security > Location Services > time-tracker-location"
