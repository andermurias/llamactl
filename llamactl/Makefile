# llamactl Makefile
BINARY      = llamactl
INSTALL_DIR = /opt/homebrew/bin
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     = -ldflags "-X github.com/andermurias/llamactl/cmd.Version=$(VERSION) -s -w"

.PHONY: build install clean test test-api lint release help

## build: compile the binary to ./llamactl
build:
	go build $(LDFLAGS) -o $(BINARY) .

## install: build and install to $(INSTALL_DIR)/llamactl
install: build
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "\u2713 Installed $(VERSION) to $(INSTALL_DIR)/$(BINARY)"

## test: run unit tests
test:
	go test -v -race ./...

## test-api: run shell integration tests against a live llama-swap instance
test-api:
	@echo "Running API integration tests..."
	bash ../scripts/test-api.sh --fast

## lint: run go vet and staticcheck
lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 \
		&& staticcheck ./... \
		|| echo "(staticcheck not installed - run: go install honnef.co/go/tools/cmd/staticcheck@latest)"

## release: tag and push to trigger GitHub Actions release workflow
release:
	@[ "$(TAG)" ] || (echo "Usage: make release TAG=v1.2.3" && exit 1)
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	@echo "\u2713 Tagged $(TAG) - GitHub Actions will build and publish the release"

## clean: remove compiled binary
clean:
	rm -f $(BINARY)

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
