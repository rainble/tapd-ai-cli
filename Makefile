.PHONY: build build-cross test test-integration test-coverage lint coverage clean fmt install tools

BINARY_NAME=tapd
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/studyzy/tapd-ai-cli/internal/cmd.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/tapd/

# Cross-compile. Requires GOOS and GOARCH env vars.
# Example: GOOS=linux GOARCH=arm64 make build-cross
build-cross:
	@if [ -z "$$GOOS" ] || [ -z "$$GOARCH" ]; then \
		echo "GOOS and GOARCH must be set"; exit 1; \
	fi
	@ext=""; if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
	go build $(LDFLAGS) -o $(BINARY_NAME)-$$GOOS-$$GOARCH$$ext ./cmd/tapd/

test:
	go test -race ./...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

test-integration:
	go test ./... -v -run "TestIntegration" -count=1

lint:
	go vet ./...
	@unformatted=$$(goimports -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted with goimports:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

fmt:
	gofmt -w .
	goimports -w .

coverage: test-coverage
	go tool cover -func=coverage.out

tools:
	go install golang.org/x/tools/cmd/goimports@latest

install:
	go install $(LDFLAGS) ./cmd/tapd/

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-* coverage.out
