#!/usr/bin/env bash
# uninstall.sh — Remove the time-tracker system daemon.
#
# Must be run as root.  Prompts before deleting the database.

set -euo pipefail

BINARY_DST="/usr/local/bin/time-tracker"
PLIST_DST="/Library/LaunchDaemons/com.timetracker.daemon.plist"
PLIST_LABEL="com.timetracker.daemon"
DB_DIR="/var/lib/time-tracker"
LOG_DIR="/var/log/time-tracker"
CONF_DIR="/etc/time-tracker"

info()  { echo "  [+] $*"; }
warn()  { echo "  [!] $*" >&2; }
fatal() { echo "  [✗] $*" >&2; exit 1; }

if [[ "$(id -u)" -ne 0 ]]; then
    fatal "This script must be run as root.  Use: sudo ./scripts/uninstall.sh"
fi

info "Stopping and unloading daemon"
launchctl unload -w "$PLIST_DST" 2>/dev/null || true

info "Removing plist"
rm -f "$PLIST_DST"

info "Removing binary"
rm -f "$BINARY_DST"

# Ask before deleting data.
read -r -p "  [?] Delete local database and logs? [y/N] " yn
case "$yn" in
    [Yy]*)
        info "Removing $DB_DIR"
        rm -rf "$DB_DIR"
        info "Removing $LOG_DIR"
        rm -rf "$LOG_DIR"
        ;;
    *)
        warn "Skipped — data retained at $DB_DIR and $LOG_DIR"
        ;;
esac

read -r -p "  [?] Delete config at $CONF_DIR? [y/N] " yn2
case "$yn2" in
    [Yy]*)
        info "Removing $CONF_DIR"
        rm -rf "$CONF_DIR"
        ;;
    *)
        warn "Skipped — config retained at $CONF_DIR"
        ;;
esac

echo ""
echo "Uninstall complete."
