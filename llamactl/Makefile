# llamactl Makefile
BINARY      = bin/llamactl
DIST_DIR    = bin
SCRIPTS_DIR = ../scripts
VERSION    ?= $(shell git -C .. describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     = -ldflags "-X github.com/andermurias/llamactl/cmd.Version=$(VERSION) -s -w"

.PHONY: build install clean test test-fast test-web test-api test-e2e test-inference lint dist help

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

## test: run all unit tests (includes web handler tests; ~45s due to launchd timeouts)
test:
	go test -race ./...

## test-fast: run unit tests excluding slow launchd action tests
test-fast:
	go test -race -run 'TestHandleIndex|TestHandleStatus|TestHandleLogs|TestHandleModels|TestHandleConfig|TestPostOnly|TestGetModelsInfo|TestFormatBytes|TestCheck|TestNormalize' ./...

## test-web: run only the web handler tests
test-web:
	go test -v -race ./internal/web/...

## test-api: run shell integration tests against a live llama-swap instance
test-api:
	@echo "Running API integration tests (requires llamactl start)..."
	bash ../scripts/test-api.sh --fast

## test-e2e: run fast E2E tests (health + model listing + llamactl API, no inference, ~5s)
test-e2e:
	go test -v -tags e2e -timeout 60s \
		-run 'TestLlamaSwap_|TestLlamaCtlWeb_|TestModels_|TestLlamaCtlAPI_' \
		./test/e2e/...

## test-inference: run inference E2E tests — loads ≤14B models sequentially (~3 min)
## Tip: set LLAMACTL_TEST_LARGE=1 to also include >14B models (may need -timeout 1800s)
test-inference:
	go test -v -tags e2e -timeout 600s \
		-run 'TestInference_' \
		./test/e2e/...

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
