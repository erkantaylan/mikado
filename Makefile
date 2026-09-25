# All code lives under src/: the Go module at src/, the Vite app at src/web/.
GO      ?= $(shell command -v go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
BUN     ?= bun
WEB     := src/web
DIST    := src/internal/web/dist
BIN     := bin/mikado
ADDR    ?= 127.0.0.1:47291
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX  ?= $(HOME)/.local
UNIT    := $(HOME)/.config/systemd/user/mikado.service

.DEFAULT_GOAL := help
.PHONY: help tools deps dev-web build-web build run check test clean \
	install update uninstall install-service uninstall-service \
	start stop restart status logs

# Lists every target that has a `## ` comment after its name.
help: ## show this list
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---- install and update ----------------------------------------------------

# Fails early, with what to install, instead of halfway through a build.
tools:
	@test -x "$(GO)" || command -v "$(GO)" >/dev/null || { echo "Go not found: install it from https://go.dev/dl (or pass GO=/path/to/go)"; exit 1; }
	@command -v $(BUN) >/dev/null || { echo "bun not found: install it with 'curl -fsSL https://bun.sh/install | bash'"; exit 1; }

# Installs the binary to ~/.local/bin (on PATH), refreshes the agent skill, and
# restarts the user service if it is running, so an upgrade is one command.
install: build ## build and install to ~/.local/bin, refresh the skill, restart the service
	install -Dm755 $(BIN) $(PREFIX)/bin/mikado
	$(PREFIX)/bin/mikado skill install
	@if systemctl --user is-active --quiet mikado; then systemctl --user restart mikado && echo "restarted the mikado service"; fi
	@echo "installed $(VERSION) at $(PREFIX)/bin/mikado"

# Re-runs make after the pull so the install uses the pulled Makefile and version.
update: ## pull the latest master and install it
	git pull --ff-only
	$(MAKE) install

# Leaves the data (quests, deeds) in ~/.local/share/mikado; delete it by hand to wipe them.
uninstall: uninstall-service ## remove the binary, the skill and the service (keeps your data)
	@skill=$$($(PREFIX)/bin/mikado skill path 2>/dev/null); \
	if [ -n "$$skill" ] && grep -qF 'mikado skill install' "$$skill" 2>/dev/null; then \
		rm -f "$$skill" && rmdir --ignore-fail-on-non-empty "$$(dirname "$$skill")" && echo "removed $$skill"; \
	fi
	rm -f $(PREFIX)/bin/mikado
	@echo "data kept in $${XDG_DATA_HOME:-$(HOME)/.local/share}/mikado"

# ---- service -----------------------------------------------------------------

# Runs `mikado serve` as a systemd user service, started at login (and at boot
# when lingering is on).
install-service: install ## install, then run mikado as a systemd user service
	install -Dm644 contrib/mikado.service $(UNIT)
	systemctl --user daemon-reload
	systemctl --user enable --now mikado
	@systemctl --user --no-pager status mikado | head -3

uninstall-service: ## stop and remove the systemd user service
	@if [ -f $(UNIT) ]; then \
		systemctl --user disable --now mikado; \
		rm -f $(UNIT); \
		systemctl --user daemon-reload; \
		echo "removed the mikado service"; \
	fi

start: ## start the service
	systemctl --user start mikado
stop: ## stop the service
	systemctl --user stop mikado
restart: ## restart the service
	systemctl --user restart mikado
status: ## show whether the service is running
	@systemctl --user --no-pager status mikado
logs: ## follow the service's log
	journalctl --user -u mikado -f

# ---- develop -----------------------------------------------------------------

deps: tools $(WEB)/node_modules

$(WEB)/node_modules: $(WEB)/package.json $(WEB)/bun.lock
	cd $(WEB) && $(BUN) install --frozen-lockfile
	@touch $@

run: build ## build and serve from the checkout (ADDR=host:port), without installing
	./$(BIN) serve --addr $(ADDR)

# Vite dev server; proxies /api to the Go server at $(ADDR) (run `make run` alongside).
dev-web: deps ## Vite dev server with HMR (run `make run` alongside)
	cd $(WEB) && MIKADO_API=http://$(ADDR) $(BUN) run dev

# Builds the frontend into the Go package that embeds it. .gitkeep is restored
# because Vite empties the directory and go:embed needs it to exist.
build-web: deps
	cd $(WEB) && $(BUN) run build
	@touch $(DIST)/.gitkeep

# CGO_ENABLED=0: one static executable (the SQLite driver is pure Go).
build: build-web ## build bin/mikado with the frontend embedded
	@mkdir -p $(dir $(BIN))
	cd src && CGO_ENABLED=0 $(GO) build -ldflags "-X main.version=$(VERSION)" -o ../$(BIN) ./cmd/mikado

check: deps ## tsc --noEmit and go vet
	cd $(WEB) && $(BUN) x tsc -b --noEmit
	cd src && $(GO) vet ./...

# Go unit tests (the store runs against a temp SQLite file and a fake GitHub).
test: tools ## go test
	cd src && $(GO) test ./...

clean: ## remove build output
	rm -rf bin
	find $(DIST) -mindepth 1 ! -name .gitkeep -delete
