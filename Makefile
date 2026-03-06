# Makefile — build, install, and manage the time-tracker daemon.
#
# Typical workflow on a fresh machine:
#
#   make build          # compile the binary
#   sudo make install   # install binary + plist, load daemon
#
# Targets that touch /Library/LaunchDaemons require sudo.

# ── Variables ──────────────────────────────────────────────────────────────────
BINARY       := time-tracker
BIN_DIR      := ./bin
CMD_PATH     := ./cmd/tracker
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
# Embed Info.plist so macOS TCC can show the location permission dialog.
LOC_LDFLAGS := -ldflags "-s -w -extldflags '-sectcreate __TEXT __info_plist $(CURDIR)/$(LOC_INFO_PLIST)'"

# CGo is required for the macOS IOKit idle-time probe.
CGO_ENABLED := 1
GOOS        := darwin
GOARCH      ?= arm64          # override with GOARCH=amd64 for Intel Macs

LDFLAGS := -ldflags "-s -w"   # strip debug info → smaller binary

# Values baked into the binary at build time (no .env file needed on target).
# Usage: make build-prod SYNC_API_URL=https://... SYNC_API_KEY=mytoken
SYNC_API_URL ?=
SYNC_API_KEY ?=
DB_PATH      ?= /var/lib/time-tracker/tracker.db
LOG_PATH     ?= /var/log/time-tracker

CFG_PKG := github.com/vivek/time-tracker/internal/config
PROD_LDFLAGS := -ldflags "-s -w \
  -X '$(CFG_PKG).DefaultSyncAPIURL=$(SYNC_API_URL)' \
  -X '$(CFG_PKG).DefaultSyncAPIKey=$(SYNC_API_KEY)' \
  -X '$(CFG_PKG).DefaultDBPath=$(DB_PATH)' \
  -X '$(CFG_PKG).DefaultLogPath=$(LOG_PATH)'"

.PHONY: all build build-location sign-location build-prod clean install uninstall reload status logs tidy vet

# ── Default target ─────────────────────────────────────────────────────────────
all: build sign-location

# ── Build ──────────────────────────────────────────────────────────────────────
build:
	@echo "Building $(BINARY) (GOARCH=$(GOARCH))…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD_PATH)
	@echo "→ $(BIN_DIR)/$(BINARY)"

# Location helper: runs as a user LaunchAgent to capture GPS via CoreLocation.
build-location:
	@echo "Building $(LOC_BINARY) (GOARCH=$(GOARCH))…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
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
	@echo "Building $(BINARY) (prod, GOARCH=$(GOARCH))…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(PROD_LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD_PATH)
	@echo "→ $(BIN_DIR)/$(BINARY)"

# Cross-compile for Intel Mac (useful when packaging on Apple Silicon).
build-amd64:
	@echo "Building $(BINARY) (GOARCH=amd64)…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-amd64 $(CMD_PATH)
	@echo "→ $(BIN_DIR)/$(BINARY)-amd64"

# Universal binary (runs natively on both Apple Silicon and Intel).
build-universal:
	@echo "Building universal binary…"
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-arm64 $(CMD_PATH)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-amd64 $(CMD_PATH)
	lipo -create -output $(BIN_DIR)/$(BINARY)-universal \
		$(BIN_DIR)/$(BINARY)-arm64 \
		$(BIN_DIR)/$(BINARY)-amd64
	@rm -f $(BIN_DIR)/$(BINARY)-arm64 $(BIN_DIR)/$(BINARY)-amd64
	@echo "→ $(BIN_DIR)/$(BINARY)-universal"

# ── Dependency management ──────────────────────────────────────────────────────
tidy:
	go mod tidy

vet:
	go vet ./...

# ── Install / uninstall ────────────────────────────────────────────────────────
install: build sign-location
	@echo "Installing daemon (requires root)…"
	sudo bash scripts/install.sh $(BIN_DIR)/$(BINARY) $(BIN_DIR)/$(LOC_BINARY)

uninstall:
	@echo "Removing daemon (requires root)…"
	sudo bash scripts/uninstall.sh

# ── Daemon management ──────────────────────────────────────────────────────────
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

# ── Dev helpers ────────────────────────────────────────────────────────────────
# Run locally (reads .env from the current directory).
run: build
	ENV_FILE=./.env $(BIN_DIR)/$(BINARY)

clean:
	rm -rf $(BIN_DIR)
