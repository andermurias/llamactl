# llamactl Makefile
BINARY      = bin/llamactl
DIST_DIR    = bin
SCRIPTS_DIR = ../scripts
VERSION    ?= $(shell git -C .. describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     = -ldflags "-X github.com/andermurias/llamactl/cmd.Version=$(VERSION) -s -w"

.PHONY: build install clean test test-api lint dist help

## build: compile the binary to ./bin/llamactl (with ad-hoc code signing)
build:
	@mkdir -p $(DIST_DIR)
	go build $(LDFLAGS) -o $(BINARY) .
	codesign --force -s - $(BINARY)
	@echo "✓ Built $(VERSION) → $(BINARY)"

## install: build and deploy to ../scripts/llamactl (the PATH-linked binary)
install: build
	cp $(BINARY) $(SCRIPTS_DIR)/llamactl
	codesign --force -s - $(SCRIPTS_DIR)/llamactl
	@echo "✓ Installed $(VERSION) to $(SCRIPTS_DIR)/llamactl"

## dist: build, install, and stage bin/llamactl for git commit
dist: install
	@echo "✓ Ready to commit — run: git add llamactl/bin/llamactl && git commit"

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

## clean: remove compiled binary
clean:
	rm -f $(BINARY)

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
