# llamactl Makefile

BINARY      = llamactl
INSTALL_DIR = /opt/homebrew/bin
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     = -ldflags "-X github.com/andermurias/llamactl/cmd.Version=$(VERSION) -s -w"

.PHONY: build install clean test lint

## build: compile the binary to ./llamactl
build:
	go build $(LDFLAGS) -o $(BINARY) .

## install: build and install to $(INSTALL_DIR)
install: build
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "✓ Installed to $(INSTALL_DIR)/$(BINARY)"

## test: run unit tests
test:
	go test ./...

## test-api: run integration tests against a live llama-swap instance
test-api:
	@echo "Running API integration tests..."
	bash ../scripts/test-api.sh --fast

## lint: run go vet and staticcheck
lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "(staticcheck not installed — run: go install honnef.co/go/tools/cmd/staticcheck@latest)"

## clean: remove built binary
clean:
	rm -f $(BINARY)

## help: show available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
