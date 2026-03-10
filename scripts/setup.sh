#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  Time Tracker — Unified Setup Script                                   ║
# ║  Install or uninstall with a single command:                           ║
# ║                                                                        ║
# ║  Install:                                                              ║
# ║    /bin/bash -c "$(curl -fsSL <RELEASE_URL>/setup.sh)"                 ║
# ║                                                                        ║
# ║  Uninstall:                                                            ║
# ║    /bin/bash -c "$(curl -fsSL <RELEASE_URL>/setup.sh)" -- --uninstall  ║
# ╚══════════════════════════════════════════════════════════════════════════╝
set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────────────
# Defaults to downloading from the GitHub release page
RELEASE_URL="${RELEASE_URL:-https://github.com/vivek-pk/time-tracker/releases/latest/download}"
VERSION="${VERSION:-latest}"

BINARY_DST="/usr/local/bin/time-tracker"
LOC_DST="/usr/local/bin/time-tracker-location"
DAEMON_PLIST="/Library/LaunchDaemons/com.timetracker.daemon.plist"
DAEMON_LABEL="com.timetracker.daemon"
AGENT_PLIST="/Library/LaunchAgents/com.timetracker.locationhelper.plist"
AGENT_LABEL="com.timetracker.locationhelper"
ENV_DST="/etc/time-tracker/.env"
DB_DIR="/var/lib/time-tracker"
LOG_DIR="/var/log/time-tracker"
CONF_DIR="/etc/time-tracker"

# ── Colors & Symbols ─────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
    BOLD="\033[1m"
    DIM="\033[2m"
    RESET="\033[0m"
    RED="\033[38;5;196m"
    GREEN="\033[38;5;82m"
    YELLOW="\033[38;5;220m"
    BLUE="\033[38;5;75m"
    CYAN="\033[38;5;80m"
    MAGENTA="\033[38;5;177m"
    WHITE="\033[38;5;255m"
    GRAY="\033[38;5;245m"
    BG_GREEN="\033[48;5;22m"
    BG_RED="\033[48;5;52m"
    BG_BLUE="\033[48;5;24m"
else
    BOLD="" DIM="" RESET="" RED="" GREEN="" YELLOW="" BLUE=""
    CYAN="" MAGENTA="" WHITE="" GRAY="" BG_GREEN="" BG_RED="" BG_BLUE=""
fi

SYM_CHECK="✓"
SYM_CROSS="✗"
SYM_ARROW="→"
SYM_WARN="⚠"
SYM_DOT="●"
SYM_GEAR="⚙"
SYM_LOCK="🔒"
SYM_ROCKET="🚀"
SYM_CLOCK="⏱"
SYM_PIN="📍"
SYM_ID="🆔"
SYM_PARTY="🎉"

# ── UI Helpers ────────────────────────────────────────────────────────────────
clear_line() { printf "\r\033[K"; }

print_banner() {
    echo ""
    printf "${BLUE}${BOLD}"
    cat << 'BANNER'
     ██████╗ ██████╗ ██████╗ ██████╗ ██╗     ███████╗
    ██╔════╝██╔═══██╗██╔══██╗██╔══██╗██║     ██╔════╝
    ██║     ██║   ██║██║  ██║██║  ██║██║     █████╗
    ██║     ██║   ██║██║  ██║██║  ██║██║     ██╔══╝
    ╚██████╗╚██████╔╝██████╔╝██████╔╝███████╗███████╗
     ╚═════╝ ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝╚══════╝
BANNER
    printf "${RESET}"
    echo ""
    printf "    ${DIM}${GRAY}Attendance & Activity Monitoring System${RESET}\n"
    echo ""
}

print_divider() {
    printf "  ${GRAY}────────────────────────────────────────────────────────────────${RESET}\n"
}

step_start() {
    local step_num="$1" total="$2" msg="$3"
    printf "  ${BOLD}${BLUE}[%d/%d]${RESET} ${WHITE}%s${RESET}" "$step_num" "$total" "$msg"
    # Show spinner
    _spin &
    SPIN_PID=$!
    disown $SPIN_PID 2>/dev/null
}

step_done() {
    local msg="${1:-}"
    kill "$SPIN_PID" 2>/dev/null; wait "$SPIN_PID" 2>/dev/null || true
    clear_line
    local step_num="$2" total="$3" label="$4"
    printf "  ${BOLD}${GREEN}[%d/%d]${RESET} ${WHITE}%s ${GREEN}${SYM_CHECK}${RESET}" "$step_num" "$total" "$label"
    if [[ -n "$msg" ]]; then
        printf " ${DIM}${GRAY}%s${RESET}" "$msg"
    fi
    echo ""
}

step_warn() {
    local msg="$1"
    kill "$SPIN_PID" 2>/dev/null; wait "$SPIN_PID" 2>/dev/null || true
    clear_line
    local step_num="$2" total="$3" label="$4"
    printf "  ${BOLD}${YELLOW}[%d/%d]${RESET} ${WHITE}%s ${YELLOW}${SYM_WARN}${RESET}" "$step_num" "$total" "$label"
    printf " ${DIM}${YELLOW}%s${RESET}\n" "$msg"
}

step_fail() {
    local msg="$1"
    kill "$SPIN_PID" 2>/dev/null; wait "$SPIN_PID" 2>/dev/null || true
    clear_line
    local step_num="$2" total="$3" label="$4"
    printf "  ${BOLD}${RED}[%d/%d]${RESET} ${WHITE}%s ${RED}${SYM_CROSS}${RESET}" "$step_num" "$total" "$label"
    printf " ${RED}%s${RESET}\n" "$msg"
}

_spin() {
    local frames=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
    local i=0
    while true; do
        printf "\r  ${CYAN}${frames[$i]}${RESET} "
        # Move cursor back to overwrite only the spinner
        printf "\033[3D"
        i=$(( (i + 1) % ${#frames[@]} ))
        sleep 0.08
    done
}

# ── Machine ID (matches Go binary: IOPlatformSerialNumber) ────────────────────
get_machine_id() {
    local serial
    serial=$(ioreg -rd1 -c IOPlatformExpertDevice 2>/dev/null \
        | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
    if [[ -z "$serial" ]]; then
        # Fallback: sanitised hostname + last two IP octets (matches Go code)
        local hostname_part
        hostname_part=$(hostname -s 2>/dev/null | tr -c 'a-zA-Z0-9-' '-')
        local ip_suffix
        ip_suffix=$(ifconfig 2>/dev/null \
            | awk '/inet / && !/127.0.0.1/{split($2,a,".");print a[3]"-"a[4];exit}')
        serial="${hostname_part}-${ip_suffix}"
    fi
    echo "$serial"
}

# ── Embedded Plist & Config Generators ────────────────────────────────────────
write_daemon_plist() {
    cat > "$DAEMON_PLIST" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.timetracker.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/time-tracker</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>ENV_FILE</key>
        <string>/etc/time-tracker/.env</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>ProcessType</key>
    <string>Background</string>
    <key>Nice</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>/var/log/time-tracker/output.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/time-tracker/error.log</string>
    <key>AbandonProcessGroup</key>
    <true/>
</dict>
</plist>
PLIST
}

write_agent_plist() {
    cat > "$AGENT_PLIST" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.timetracker.locationhelper</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Applications/time-tracker-location.app/Contents/MacOS/time-tracker-location</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>StartInterval</key>
    <integer>300</integer>
    <key>ProcessType</key>
    <string>Background</string>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>/tmp/time-tracker-location.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/time-tracker-location-error.log</string>
</dict>
</plist>
PLIST
}

write_default_env() {
    cat > "$ENV_DST" << 'ENVFILE'
# /etc/time-tracker/.env  (OPTIONAL OVERRIDE FILE)
# All configuration is embedded in the binary (config.json).
# Uncomment variables below ONLY if you need to override them on this specific machine.
# Edit this file and reload: sudo launchctl kickstart -k system/com.timetracker.daemon

# ── Required ──────────────────────────────────────────────────────────────────
# SYNC_API_URL=https://your-api-endpoint.example.com/attendance

# ── Authentication ────────────────────────────────────────────────────────────
# SYNC_API_KEY=

# ── Sync schedule ─────────────────────────────────────────────────────────────
# MORNING_SYNC_HOUR=6
# EVENING_SYNC_HOUR=18
# EVENING_SYNC_MINUTE=30

# ── Activity detection ────────────────────────────────────────────────────────
# IDLE_THRESHOLD_MINUTES=5
# POLL_INTERVAL_SECONDS=30

# ── Storage ───────────────────────────────────────────────────────────────────
# DB_PATH=/var/lib/time-tracker/tracker.db
# LOG_PATH=/var/log/time-tracker
# RETENTION_DAYS=3
# SYNC_TIMEOUT_SECONDS=30
ENVFILE
}

# ══════════════════════════════════════════════════════════════════════════════
#  INSTALL
# ══════════════════════════════════════════════════════════════════════════════
do_install() {
    local TOTAL_STEPS=9

    print_banner

    printf "  ${WHITE}${BOLD}This script will install Time Tracker on this Mac.${RESET}\n\n"
    printf "  ${GRAY}${SYM_GEAR}  System daemon   ${DIM}(activity monitoring, runs as root)${RESET}\n"
    printf "  ${GRAY}${SYM_PIN}  Location helper  ${DIM}(GPS capture, runs in user session)${RESET}\n"
    printf "  ${GRAY}${SYM_LOCK}  Tamper-proof     ${DIM}(users cannot stop the daemon)${RESET}\n"
    echo ""
    print_divider

    # ── Pre-flight: need root ──
    if [[ "$(id -u)" -ne 0 ]]; then
        echo ""
        printf "  ${YELLOW}${SYM_WARN}  Root access required.${RESET}\n"
        printf "  ${DIM}${GRAY}Run:  sudo /bin/bash -c \"\$(curl -fsSL <URL>/setup.sh)\"${RESET}\n\n"
        exit 1
    fi

    echo ""

    # ── Step 1: Check prerequisites ──
    local S=1
    step_start $S $TOTAL_STEPS "Checking prerequisites..."
    sleep 0.4
    if [[ "$(uname)" != "Darwin" ]]; then
        step_fail "macOS is required" $S $TOTAL_STEPS "Checking prerequisites"
        exit 1
    fi
    local ARCH
    ARCH=$(uname -m)
    if [[ "$ARCH" != "arm64" && "$ARCH" != "x86_64" ]]; then
        step_fail "Unsupported architecture: $ARCH" $S $TOTAL_STEPS "Checking prerequisites"
        exit 1
    fi
    step_done "macOS $(sw_vers -productVersion) / $ARCH" $S $TOTAL_STEPS "Checking prerequisites"

    # ── Step 2: Download or locate binaries ──
    S=2
    step_start $S $TOTAL_STEPS "Preparing binaries..."
    local TMP_DIR
    TMP_DIR=$(mktemp -d)
    trap "rm -rf '$TMP_DIR'" EXIT

    local BINARY_SRC="" LOC_APP_SRC=""

    # Priority 1: Binaries next to this script (local repo build)
    local SCRIPT_DIR
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd)" || SCRIPT_DIR=""
    if [[ -n "$SCRIPT_DIR" && -f "$SCRIPT_DIR/../bin/time-tracker" && -d "$SCRIPT_DIR/../bin/time-tracker-location.app" ]]; then
        BINARY_SRC="$SCRIPT_DIR/../bin/time-tracker"
        LOC_APP_SRC="$SCRIPT_DIR/../bin/time-tracker-location.app"
        step_done "using local build" $S $TOTAL_STEPS "Preparing binaries"
    # Priority 2: Download from RELEASE_URL
    elif [[ -n "$RELEASE_URL" ]]; then
        local tar_name="time-tracker-darwin-universal.tar.gz"
        if ! curl -fsSL "${RELEASE_URL}/${tar_name}" -o "$TMP_DIR/$tar_name" 2>/dev/null; then
            step_fail "download failed from ${RELEASE_URL}" $S $TOTAL_STEPS "Preparing binaries"
            exit 1
        fi
        tar -xzf "$TMP_DIR/$tar_name" -C "$TMP_DIR" 2>/dev/null
        BINARY_SRC="$TMP_DIR/time-tracker"
        LOC_APP_SRC="$TMP_DIR/time-tracker-location.app"
        if [[ ! -f "$BINARY_SRC" || ! -d "$LOC_APP_SRC" ]]; then
            step_fail "archive missing binaries" $S $TOTAL_STEPS "Preparing binaries"
            exit 1
        fi
        step_done "downloaded $VERSION" $S $TOTAL_STEPS "Preparing binaries"
    # Priority 3: Already installed? (upgrade path)
    elif [[ -f "$BINARY_DST" && -d "/Applications/time-tracker-location.app" ]]; then
        step_warn "no RELEASE_URL set — keeping existing binaries" $S $TOTAL_STEPS "Preparing binaries"
        BINARY_SRC="" # skip copy
    else
        step_fail "set RELEASE_URL or build locally first (make build sign-location)" $S $TOTAL_STEPS "Preparing binaries"
        printf "\n  ${DIM}${GRAY}Example:${RESET}\n"
        printf "  ${CYAN}RELEASE_URL=https://github.com/org/repo/releases/latest/download sudo bash setup.sh${RESET}\n\n"
        exit 1
    fi

    # ── Step 3: Create directories ──
    S=3
    step_start $S $TOTAL_STEPS "Creating directories..."
    sleep 0.3
    for dir in "$DB_DIR" "$LOG_DIR" "$CONF_DIR"; do
        mkdir -p "$dir"
        chown root:wheel "$dir"
        chmod 750 "$dir"
    done
    step_done "" $S $TOTAL_STEPS "Creating directories"

    # ── Step 4: Install tracker daemon ──
    S=4
    step_start $S $TOTAL_STEPS "Installing tracker daemon..."
    sleep 0.3
    if [[ -n "$BINARY_SRC" ]]; then
        cp "$BINARY_SRC" "$BINARY_DST"
        chown root:wheel "$BINARY_DST"
        chmod 755 "$BINARY_DST"
    fi
    step_done "$BINARY_DST" $S $TOTAL_STEPS "Installing tracker daemon"

    # ── Step 5: Install location helper ──
    S=5
    step_start $S $TOTAL_STEPS "Installing location helper..."
    sleep 0.3
    local LOC_APP_DIR="/Applications/time-tracker-location.app"
    if [[ -n "$BINARY_SRC" ]]; then
        rm -rf "$LOC_APP_DIR" 2>/dev/null || true
        cp -R "$LOC_APP_SRC" "$LOC_APP_DIR"
        # We re-apply an ad-hoc deep signature to match the exact identity of the host CPU's cdhash
        # If Xcode tools are not installed, it fails gracefully and uses the pre-built signature! 
        codesign -f -s - --deep "$LOC_APP_DIR" 2>/dev/null || true
    fi
    step_done "/Applications" $S $TOTAL_STEPS "Installing location helper"

    # ── Step 6: Request location permission ──
    S=6
    step_start $S $TOTAL_STEPS "Requesting location permission..."
    kill "$SPIN_PID" 2>/dev/null; wait "$SPIN_PID" 2>/dev/null || true
    clear_line

    local CURRENT_USER
    CURRENT_USER=$(stat -f '%Su' /dev/console 2>/dev/null) || CURRENT_USER=""

    if [[ -n "$CURRENT_USER" && "$CURRENT_USER" != "root" ]]; then
        echo ""
        printf "  ${YELLOW}${BOLD}  ${SYM_PIN}  Location Services Permission${RESET}\n\n"
        printf "  ${WHITE}  A permission dialog will appear shortly.${RESET}\n"
        printf "  ${WHITE}  Please click ${BOLD}${GREEN}\"Allow\"${RESET}${WHITE} to grant location access.${RESET}\n\n"
        printf "  ${DIM}${GRAY}  Waiting for you to click Allow (auto-timeout: 30s)...${RESET}\n"
        echo ""

        # Launch as the GUI user so macOS shows the TCC dialog.
        # open -W blocks until the app exits. The location helper has a
        # built-in 30s timeout, so this will not hang indefinitely.
        sudo -u "$CURRENT_USER" open -W "$LOC_APP_DIR" 2>/dev/null || true

        # Check if a fix was written
        if [[ -f /tmp/time-tracker-location.json ]]; then
            printf "  ${BOLD}${GREEN}[%d/%d]${RESET} ${WHITE}%s ${GREEN}${SYM_CHECK}${RESET} ${DIM}${GRAY}permission granted${RESET}\n" $S $TOTAL_STEPS "Requesting location permission"
        else
            printf "  ${BOLD}${YELLOW}[%d/%d]${RESET} ${WHITE}%s ${YELLOW}${SYM_WARN}${RESET} ${DIM}${YELLOW}grant manually in System Settings → Privacy → Location Services${RESET}\n" $S $TOTAL_STEPS "Requesting location permission"
        fi
    else
        printf "  ${BOLD}${YELLOW}[%d/%d]${RESET} ${WHITE}%s ${YELLOW}${SYM_WARN}${RESET} ${DIM}${YELLOW}run 'open %s' after login to grant permission${RESET}\n" $S $TOTAL_STEPS "Requesting location permission" "$LOC_APP_DIR"
    fi


    # ── Step 7: Install configuration (optional override) ──
    S=7
    step_start $S $TOTAL_STEPS "Installing configuration..."
    sleep 0.3
    if [[ -f "$ENV_DST" ]]; then
        step_warn "config exists — not overwriting" $S $TOTAL_STEPS "Installing configuration"
    else
        write_default_env
        chown root:wheel "$ENV_DST"
        chmod 640 "$ENV_DST"
        step_done "optional override at $ENV_DST" $S $TOTAL_STEPS "Installing configuration"
    fi

    # ── Step 8: Load system daemon ──
    S=8
    step_start $S $TOTAL_STEPS "Loading system daemon..."
    write_daemon_plist
    chown root:wheel "$DAEMON_PLIST"
    chmod 644 "$DAEMON_PLIST"
    launchctl unload "$DAEMON_PLIST" 2>/dev/null || true
    launchctl load -w "$DAEMON_PLIST"
    sleep 1
    if launchctl list 2>/dev/null | grep -q "$DAEMON_LABEL"; then
        step_done "running" $S $TOTAL_STEPS "Loading system daemon"
    else
        step_warn "may need reboot" $S $TOTAL_STEPS "Loading system daemon"
    fi

    # ── Step 9: Load location agent ──
    S=9
    step_start $S $TOTAL_STEPS "Loading location agent..."
    write_agent_plist
    chown root:wheel "$AGENT_PLIST"
    chmod 644 "$AGENT_PLIST"
    local CURRENT_USER
    CURRENT_USER=$(stat -f '%Su' /dev/console 2>/dev/null) || CURRENT_USER=""
    if [[ -n "$CURRENT_USER" && "$CURRENT_USER" != "root" ]]; then
        local CURRENT_UID
        CURRENT_UID=$(id -u "$CURRENT_USER")
        launchctl bootout "gui/${CURRENT_UID}/${AGENT_LABEL}" 2>/dev/null || true
        if launchctl bootstrap "gui/${CURRENT_UID}" "$AGENT_PLIST" 2>/dev/null; then
            step_done "for user $CURRENT_USER" $S $TOTAL_STEPS "Loading location agent"
        else
            step_warn "log out & back in to activate" $S $TOTAL_STEPS "Loading location agent"
        fi
    else
        step_warn "will activate on next login" $S $TOTAL_STEPS "Loading location agent"
    fi

    # ── Success ──
    echo ""
    print_divider
    show_success_box
}

# ── Success Box with Machine ID ───────────────────────────────────────────────
show_success_box() {
    local machine_id
    machine_id=$(get_machine_id)

    echo ""
    echo ""
    printf "  ${GREEN}${BOLD}  ============================================${RESET}\n"
    printf "  ${GREEN}${BOLD}    INSTALLATION COMPLETE                     ${RESET}\n"
    printf "  ${GREEN}${BOLD}  ============================================${RESET}\n"
    echo ""
    echo ""

    # Machine ID — simple, reliable display
    printf "  ${WHITE}${BOLD}  YOUR MACHINE ID:${RESET}\n"
    echo ""
    printf "  ${CYAN}  ┌──────────────────────────────────────────┐${RESET}\n"
    printf "  ${CYAN}  │                                          │${RESET}\n"
    printf "  ${CYAN}  │${RESET}     ${YELLOW}${BOLD}>>>   %-12s   <<<${RESET}     ${CYAN}        │${RESET}\n" "$machine_id"
    printf "  ${CYAN}  │                                          │${RESET}\n"
    printf "  ${CYAN}  └──────────────────────────────────────────┘${RESET}\n"
    echo ""
    echo ""

    # Action required
    printf "  ${YELLOW}${BOLD}  !! ACTION REQUIRED !!${RESET}\n"
    echo ""
    printf "  ${WHITE}  Please share the Machine ID above with your admin${RESET}\n"
    printf "  ${WHITE}  and ask them to ${BOLD}${CYAN}add this device to the HRMS system${RESET}${WHITE}.${RESET}\n"
    echo ""
    printf "  ${DIM}${GRAY}  Without HRMS registration, attendance data from this${RESET}\n"
    printf "  ${DIM}${GRAY}  machine will not be linked to your employee profile.${RESET}\n"
    echo ""
    print_divider
    echo ""

    # Quick reference
    printf "  ${BOLD}${WHITE}  Quick Reference${RESET}\n"
    echo ""
    printf "  ${GRAY}  Config file   ${WHITE}/etc/time-tracker/.env ${DIM}(optional override)${RESET}\n"
    printf "  ${GRAY}  Logs          ${WHITE}${LOG_DIR}/output.log${RESET}\n"
    printf "  ${GRAY}  Database      ${WHITE}${DB_DIR}/tracker.db${RESET}\n"
    printf "  ${GRAY}  Machine ID    ${CYAN}${BOLD}${machine_id}${RESET}\n"
    echo ""
    printf "  ${GRAY}  Reload daemon:${RESET}\n"
    printf "  ${DIM}  sudo launchctl kickstart -k system/${DAEMON_LABEL}${RESET}\n"
    echo ""
    print_divider
    echo ""
}

# ══════════════════════════════════════════════════════════════════════════════
#  UNINSTALL
# ══════════════════════════════════════════════════════════════════════════════
do_uninstall() {
    local TOTAL_STEPS=5

    print_banner

    printf "  ${RED}${BOLD}This will remove Time Tracker from this Mac.${RESET}\n\n"
    print_divider

    if [[ "$(id -u)" -ne 0 ]]; then
        echo ""
        printf "  ${YELLOW}${SYM_WARN}  Root access required.${RESET}\n"
        printf "  ${DIM}${GRAY}Run:  sudo /bin/bash -c \"\$(curl -fsSL <URL>/setup.sh)\" -- --uninstall${RESET}\n\n"
        exit 1
    fi

    echo ""

    # ── Step 1: Stop daemons ──
    local S=1
    step_start $S $TOTAL_STEPS "Stopping daemons..."
    launchctl unload -w "$DAEMON_PLIST" 2>/dev/null || true
    local CURRENT_USER
    CURRENT_USER=$(stat -f '%Su' /dev/console 2>/dev/null) || CURRENT_USER=""
    if [[ -n "$CURRENT_USER" && "$CURRENT_USER" != "root" ]]; then
        local CURRENT_UID
        CURRENT_UID=$(id -u "$CURRENT_USER")
        launchctl bootout "gui/${CURRENT_UID}/${AGENT_LABEL}" 2>/dev/null || true
    fi
    sleep 1
    step_done "" $S $TOTAL_STEPS "Stopping daemons"

    # ── Step 2: Remove binaries ──
    S=2
    step_start $S $TOTAL_STEPS "Removing binaries..."
    sleep 0.3
    rm -f "$BINARY_DST" "$LOC_DST"
    step_done "" $S $TOTAL_STEPS "Removing binaries"

    # ── Step 3: Remove plists ──
    S=3
    step_start $S $TOTAL_STEPS "Removing launch configurations..."
    sleep 0.3
    rm -f "$DAEMON_PLIST" "$AGENT_PLIST"
    step_done "" $S $TOTAL_STEPS "Removing launch configurations"

    # ── Step 4: Data & logs ──
    S=4
    step_start $S $TOTAL_STEPS "Cleaning data & logs..."
    kill "$SPIN_PID" 2>/dev/null; wait "$SPIN_PID" 2>/dev/null || true
    clear_line
    echo ""
    printf "  ${YELLOW}  Delete local database and logs?${RESET}\n"
    printf "  ${DIM}  ${DB_DIR}  •  ${LOG_DIR}${RESET}\n"
    printf "  ${WHITE}  [y/N]: ${RESET}"
    local yn=""
    read -r yn </dev/tty 2>/dev/null || yn="n"
    case "$yn" in
        [Yy]*)
            rm -rf "$DB_DIR" "$LOG_DIR"
            rm -f /tmp/time-tracker-location.json /tmp/time-tracker-location.log /tmp/time-tracker-location-error.log
            printf "  ${GREEN}  ${SYM_CHECK} Data removed${RESET}\n"
            ;;
        *)
            printf "  ${GRAY}  ${SYM_ARROW} Data retained${RESET}\n"
            ;;
    esac
    echo ""

    # ── Step 5: Config ──
    S=5
    printf "  ${YELLOW}  Delete configuration?${RESET}\n"
    printf "  ${DIM}  ${CONF_DIR}${RESET}\n"
    printf "  ${WHITE}  [y/N]: ${RESET}"
    local yn2=""
    read -r yn2 </dev/tty 2>/dev/null || yn2="n"
    case "$yn2" in
        [Yy]*)
            rm -rf "$CONF_DIR"
            printf "  ${GREEN}  ${SYM_CHECK} Configuration removed${RESET}\n"
            ;;
        *)
            printf "  ${GRAY}  ${SYM_ARROW} Configuration retained${RESET}\n"
            ;;
    esac

    echo ""
    print_divider
    echo ""
    printf "  ${GREEN}${BOLD}  ${SYM_CHECK}  Time Tracker has been uninstalled.${RESET}\n\n"
    print_divider
    echo ""
}

# ══════════════════════════════════════════════════════════════════════════════
#  MAIN
# ══════════════════════════════════════════════════════════════════════════════
main() {
    local action="install"

    # Handle `bash -c "..." --uninstall` where it gets assigned to $0
    if [[ "$0" == "--uninstall" || "$0" == "uninstall" || "$0" == "-u" ]]; then
        action="uninstall"
    fi

    for arg in "$@"; do
        case "$arg" in
            --uninstall|uninstall|-u)
                action="uninstall"
                ;;
            --help|-h)
                echo ""
                printf "${BOLD}Time Tracker Setup${RESET}\n\n"
                printf "Usage:\n"
                printf "  ${CYAN}setup.sh${RESET}              Install Time Tracker\n"
                printf "  ${CYAN}setup.sh --uninstall${RESET}  Remove Time Tracker\n"
                printf "  ${CYAN}setup.sh --help${RESET}       Show this help\n"
                echo ""
                printf "Environment variables:\n"
                printf "  ${CYAN}RELEASE_URL${RESET}   URL to download pre-built binaries from\n"
                printf "  ${CYAN}VERSION${RESET}       Version tag to download (default: latest)\n"
                echo ""
                exit 0
                ;;
        esac
    done

    case "$action" in
        install)    do_install   ;;
        uninstall)  do_uninstall ;;
    esac
}

main "$@"
