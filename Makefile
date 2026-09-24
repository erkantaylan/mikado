# All code lives under src/: the Go module at src/, the Vite app at src/web/.
GO      ?= $(shell command -v go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
BUN     ?= bun
WEB     := src/web
DIST    := src/internal/web/dist
BIN     := bin/mikado
ADDR    ?= 127.0.0.1:47291
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help deps dev-web build-web build run check test clean

help:
	@echo "targets: deps dev-web build-web build run check test clean"

deps: $(WEB)/node_modules

$(WEB)/node_modules: $(WEB)/package.json $(WEB)/bun.lock
	cd $(WEB) && $(BUN) install --frozen-lockfile
	@touch $@

# Vite dev server; proxies /api to the Go server at $(ADDR) (run `make run` alongside).
dev-web: deps
	cd $(WEB) && MIKADO_API=http://$(ADDR) $(BUN) run dev

# Builds the frontend into the Go package that embeds it. .gitkeep is restored
# because Vite empties the directory and go:embed needs it to exist.
build-web: deps
	cd $(WEB) && $(BUN) run build
	@touch $(DIST)/.gitkeep

# CGO_ENABLED=0: one static executable (the SQLite driver is pure Go).
build: build-web
	@mkdir -p $(dir $(BIN))
	cd src && CGO_ENABLED=0 $(GO) build -ldflags "-X main.version=$(VERSION)" -o ../$(BIN) ./cmd/mikado

run: build
	./$(BIN) serve --addr $(ADDR)

check: deps
	cd $(WEB) && $(BUN) x tsc -b --noEmit
	cd src && $(GO) vet ./...

# Go unit tests (the store runs against a temp SQLite file and a fake GitHub).
test:
	cd src && $(GO) test ./...

clean:
	rm -rf bin
	find $(DIST) -mindepth 1 ! -name .gitkeep -delete

# Installs the binary to ~/.local/bin (on PATH), refreshes the agent skill, and
# restarts the user service if it is running, so an upgrade is one command.
PREFIX  ?= $(HOME)/.local
UNIT    := $(HOME)/.config/systemd/user/mikado.service

install: build
	install -Dm755 $(BIN) $(PREFIX)/bin/mikado
	$(PREFIX)/bin/mikado skill install
	@if systemctl --user is-active --quiet mikado; then systemctl --user restart mikado && echo "restarted the mikado service"; fi

# Runs `mikado serve` as a systemd user service, started at login (and at boot
# when lingering is on).
install-service: install
	install -Dm644 contrib/mikado.service $(UNIT)
	systemctl --user daemon-reload
	systemctl --user enable --now mikado
	@systemctl --user --no-pager status mikado | head -3
