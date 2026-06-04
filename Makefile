# Makefile — build, install, and manage the time-tracker daemon.
#
# Typical workflow on a fresh machine:
#
#   make build          # compile the binary (auto-detects OS)
#   sudo make install   # install binary + service (macOS/Linux)
#
# Cross-compile for other platforms:
#
#   make build-linux    # Linux amd64
#   make build-windows  # Windows amd64
#   make build-all      # All platforms

# ── Variables ──────────────────────────────────────────────────────────────────
BINARY       := time-tracker
BIN_DIR      := ./bin
CMD_PATH     := ./cmd/tracker
LOC_LINUX_CMD := ./cmd/location-helper-linux
LOC_WIN_CMD   := ./cmd/location-helper-windows
PLIST_LABEL  := com.timetracker.daemon
PLIST_DST    := /Library/LaunchDaemons/$(PLIST_LABEL).plist
INSTALL_DST  := /usr/local/bin/$(BINARY)

LOC_BINARY   := time-tracker-location
LOC_CMD_PATH := ./cmd/location-helper
LOC_AGENT_LABEL := com.timetracker.locationhelper
LOC_AGENT_SRC   := ./launchd/$(LOC_AGENT_LABEL).plist
LOC_AGENT_DST   := /Library/LaunchAgents/$(LOC_AGENT_LABEL).plist
ENTITLEMENTS    := ./entitlements/location-helper.plist
LOC_INFO_PLIST  := ./entitlements/location-helper-info.plist
# Minimum macOS version for the location helper binary.
# Set to 13.0 (Ventura) so the binary runs on Ventura and older releases.
MACOS_MIN_VERSION := 13.0
# Embed Info.plist so macOS TCC can show the location permission dialog.
LOC_LDFLAGS := -ldflags "-s -w -extldflags '-sectcreate __TEXT __info_plist $(CURDIR)/$(LOC_INFO_PLIST) -mmacosx-version-min=$(MACOS_MIN_VERSION)'"

LDFLAGS = -ldflags "-s -w -X '$(CFG_PKG).Version=$(VERSION)'"   # strip debug info → smaller binary

# Auto-detect host OS and architecture for native builds
HOST_OS    := $(shell go env GOOS)
HOST_ARCH  := $(shell go env GOARCH)

# CGo is required on macOS (IOKit idle-time probe, IOKit serial number).
# Windows and Linux idle detection are pure Go — no CGo needed.
ifeq ($(HOST_OS),darwin)
  CGO_ENABLED := 1
else
  CGO_ENABLED := 0
endif

GOOS   ?= $(HOST_OS)
GOARCH ?= $(HOST_ARCH)

# Values baked into the binary at build time (no .env file needed on target).
# Usage: make build-prod SYNC_API_URL=https://... SYNC_API_KEY=mytoken
SYNC_API_URL ?=
SYNC_API_KEY ?=
DB_PATH      ?=
LOG_PATH     ?=
VERSION      ?= 1.5.6

CFG_PKG := github.com/vivek/time-tracker/internal/config
PROD_LDFLAGS := -ldflags "-s -w \
  -X '$(CFG_PKG).DefaultSyncAPIURL=$(SYNC_API_URL)' \
  -X '$(CFG_PKG).DefaultSyncAPIKey=$(SYNC_API_KEY)' \
  -X '$(CFG_PKG).DefaultDBPath=$(DB_PATH)' \
  -X '$(CFG_PKG).DefaultLogPath=$(LOG_PATH)' \
  -X '$(CFG_PKG).Version=$(VERSION)'"

.PHONY: all build build-location sign-location build-prod build-debug clean install install-debug uninstall \
        setup setup-uninstall reload status logs tidy vet \
        build-linux build-windows build-all build-amd64 build-universal

# ── Default target ─────────────────────────────────────────────────────────────
ifeq ($(HOST_OS),darwin)
all: build sign-location
else
all: build
endif

# ── Build (auto-detects host OS) ──────────────────────────────────────────────
build:
	@echo "Building $(BINARY) (GOOS=$(GOOS) GOARCH=$(GOARCH))…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe,) $(CMD_PATH)
	@echo "→ $(BIN_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe,)"

# ── Cross-platform builds ────────────────────────────────────────────────────
build-linux:
	@echo "Building $(BINARY) (linux/amd64)…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-linux-amd64 $(CMD_PATH)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(LOC_BINARY)-linux-amd64 $(LOC_LINUX_CMD)
	@echo "→ $(BIN_DIR)/$(BINARY)-linux-amd64"
	@echo "→ $(BIN_DIR)/$(LOC_BINARY)-linux-amd64"

build-linux-arm64:
	@echo "Building $(BINARY) (linux/arm64)…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-linux-arm64 $(CMD_PATH)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(LOC_BINARY)-linux-arm64 $(LOC_LINUX_CMD)
	@echo "→ $(BIN_DIR)/$(BINARY)-linux-arm64"
	@echo "→ $(BIN_DIR)/$(LOC_BINARY)-linux-arm64"

build-windows:
	@echo "Building $(BINARY) (windows/amd64)…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-windows-amd64.exe $(CMD_PATH)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(LOC_BINARY)-windows-amd64.exe $(LOC_WIN_CMD)
	@echo "→ $(BIN_DIR)/$(BINARY)-windows-amd64.exe"
	@echo "→ $(BIN_DIR)/$(LOC_BINARY)-windows-amd64.exe"

build-windows-arm64:
	@echo "Building $(BINARY) (windows/arm64)…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-windows-arm64.exe $(CMD_PATH)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(LOC_BINARY)-windows-arm64.exe $(LOC_WIN_CMD)
	@echo "→ $(BIN_DIR)/$(BINARY)-windows-arm64.exe"
	@echo "→ $(BIN_DIR)/$(LOC_BINARY)-windows-arm64.exe"

build-all: build build-linux build-linux-arm64 build-windows build-windows-arm64
	@echo "All platform builds complete."

# ── macOS-specific targets ────────────────────────────────────────────────────

# Location helper: runs as a user LaunchAgent to capture GPS via CoreLocation.
build-location:
	@echo "Building $(LOC_BINARY) (GOARCH=$(GOARCH), min macOS=$(MACOS_MIN_VERSION))…"
	@mkdir -p $(BIN_DIR)
	MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN_VERSION) CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) \
		go build $(LOC_LDFLAGS) -o $(BIN_DIR)/$(LOC_BINARY) $(LOC_CMD_PATH)
	@echo "→ $(BIN_DIR)/$(LOC_BINARY) (unsigned; run scripts/make-location-app.sh to sign)"

# Sign location helper via app bundle (establishes stable cdhash for locationd)
sign-location: build-location
	@echo "Signing $(LOC_BINARY) via app bundle…"
	bash scripts/make-location-app.sh
	@echo "→ $(BIN_DIR)/$(LOC_BINARY) (signed)"

# Production build: bakes SYNC_API_URL / SYNC_API_KEY / paths into the binary.
# The binary works without any .env file on the target machine.
build-prod:
	@echo "Building $(BINARY) (prod, GOOS=$(GOOS) GOARCH=$(GOARCH))…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(PROD_LDFLAGS) -o $(BIN_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe,) $(CMD_PATH)
	@echo "→ $(BIN_DIR)/$(BINARY)$(if $(filter windows,$(GOOS)),.exe,)"

# Cross-compile for Intel Mac (useful when packaging on Apple Silicon).
build-amd64:
	@echo "Building $(BINARY) (GOARCH=amd64)…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-amd64 $(CMD_PATH)
	@echo "→ $(BIN_DIR)/$(BINARY)-amd64"

build-universal:
	@echo "Building universal binaries (arm64 + amd64)…"
	@mkdir -p $(BIN_DIR)
	# Compile ARM64
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-arm64 $(CMD_PATH)
	MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN_VERSION) CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LOC_LDFLAGS) -o $(BIN_DIR)/$(LOC_BINARY)-arm64 $(LOC_CMD_PATH)
	# Compile AMD64
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-amd64 $(CMD_PATH)
	MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN_VERSION) CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build $(LOC_LDFLAGS) -o $(BIN_DIR)/$(LOC_BINARY)-amd64 $(LOC_CMD_PATH)
	# Lipo into single universal binaries
	lipo -create -output $(BIN_DIR)/$(BINARY) $(BIN_DIR)/$(BINARY)-arm64 $(BIN_DIR)/$(BINARY)-amd64
	lipo -create -output $(BIN_DIR)/$(LOC_BINARY) $(BIN_DIR)/$(LOC_BINARY)-arm64 $(BIN_DIR)/$(LOC_BINARY)-amd64
	@rm -f $(BIN_DIR)/$(BINARY)-arm64 $(BIN_DIR)/$(BINARY)-amd64 $(BIN_DIR)/$(LOC_BINARY)-arm64 $(BIN_DIR)/$(LOC_BINARY)-amd64
	@echo "Signing universal location helper…"
	@bash scripts/make-location-app.sh $(BIN_DIR)/$(LOC_BINARY) >/dev/null
	@echo "→ $(BIN_DIR)/$(BINARY) (Universal)"
	@echo "→ $(BIN_DIR)/$(LOC_BINARY) (Universal, Signed)"

# ── Dependency management ──────────────────────────────────────────────────────
tidy:
	go mod tidy

vet:
	go vet ./...

# ── Debug build (embeds config.debug.json: localhost API + realtime_sync=true) ──
build-debug:
	@echo "Building $(BINARY)-debug (debug tags, GOOS=$(GOOS) GOARCH=$(GOARCH))…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -tags debug $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-debug$(if $(filter windows,$(GOOS)),.exe,) $(CMD_PATH)
	@echo "→ $(BIN_DIR)/$(BINARY)-debug (localhost:3000, realtime_sync=true)"

# ── Install / uninstall (platform-aware) ──────────────────────────────────────
ifeq ($(HOST_OS),darwin)
install: build sign-location
	@echo "Installing daemon (requires root)…"
	sudo bash scripts/install.sh $(BIN_DIR)/$(BINARY) $(BIN_DIR)/$(LOC_BINARY)

install-debug: build-debug sign-location
	@echo "Installing DEBUG daemon (requires root)…"
	sudo bash scripts/install.sh $(BIN_DIR)/$(BINARY)-debug $(BIN_DIR)/$(LOC_BINARY)
else ifeq ($(HOST_OS),linux)
install: build
	@echo "Installing service (requires root)…"
	sudo bash scripts/install-linux.sh $(BIN_DIR)/$(BINARY)

install-debug: build-debug
	@echo "Installing DEBUG service (requires root)…"
	sudo bash scripts/install-linux.sh $(BIN_DIR)/$(BINARY)-debug
else
install:
	@echo "On Windows, run scripts\\install-windows.ps1 as Administrator."
	@echo "See README.md for instructions."

install-debug:
	@echo "On Windows, build with: go build -tags debug ./cmd/tracker"
endif

ifeq ($(HOST_OS),darwin)
uninstall:
	@echo "Removing daemon (requires root)…"
	sudo bash scripts/uninstall.sh
else ifeq ($(HOST_OS),linux)
uninstall:
	@echo "Removing service (requires root)…"
	sudo bash scripts/uninstall-linux.sh
else
uninstall:
	@echo "On Windows, run scripts\\uninstall-windows.ps1 as Administrator."
endif

# ── Unified Setup (curl-friendly, macOS only) ─────────────────────────────────
setup: build sign-location
	sudo bash scripts/setup.sh

setup-uninstall:
	sudo bash scripts/setup.sh --uninstall

# ── Daemon management (platform-aware) ────────────────────────────────────────
ifeq ($(HOST_OS),darwin)
reload:
	@echo "Reloading daemon…"
	sudo launchctl unload $(PLIST_DST) 2>/dev/null || true
	sudo launchctl load -w $(PLIST_DST)

stop:
	@echo "Stopping daemon (will be restarted by launchd unless you unload)…"
	sudo launchctl stop $(PLIST_LABEL)

status:
	@sudo launchctl list | grep $(PLIST_LABEL) || echo "Daemon not loaded"

logs:
	@tail -f /var/log/time-tracker/output.log

logs-err:
	@tail -f /var/log/time-tracker/error.log

else ifeq ($(HOST_OS),linux)
reload:
	@echo "Reloading service…"
	sudo systemctl daemon-reload
	sudo systemctl restart time-tracker

stop:
	@echo "Stopping service…"
	sudo systemctl stop time-tracker

status:
	@systemctl status time-tracker --no-pager || true

logs:
	@journalctl -u time-tracker -f

logs-err:
	@journalctl -u time-tracker -p err -f

else
reload stop status logs logs-err:
	@echo "Use Task Scheduler to manage the TimeTracker scheduled task."
endif

# ── Dev helpers ────────────────────────────────────────────────────────────────
# Run locally (uses embedded config.json).
run: build
	$(BIN_DIR)/$(BINARY)

clean:
	rm -rf $(BIN_DIR)
