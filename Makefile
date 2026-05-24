BIN     := tmp/fafi2
GOFLAGS := --tags fts5
SERVICE := fafi.service

# Requires devbox (https://www.jetify.com/devbox) — pins the Go toolchain
# to the version declared in go.mod.
GO := devbox run -- go

.PHONY: build test lint run restart migrate clean

build: $(BIN)

$(BIN): $(shell find . -name '*.go' -not -path './tmp/*') go.mod go.sum
	@mkdir -p tmp
	$(GO) build $(GOFLAGS) -o $(BIN) fafi2

test:
	$(GO) test $(GOFLAGS) -race ./...

lint:
	devbox run -- golangci-lint run

run: build
	./$(BIN)

# Restart the user systemd service (contrib/fafi.service).
restart:
	systemctl --user restart $(SERVICE)
	systemctl --user status $(SERVICE) --no-pager --lines=5

# Build and restart — the auto-migration runs on startup, so this is the
# one-shot "deploy locally" target.
migrate: build restart

clean:
	rm -rf tmp/
