.PHONY: build install clean test lint tidy fmt check release-check

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -ldflags "-X main.version=$(VERSION)"
MISE_CACHE_DIR ?= $(CURDIR)/.cache/mise
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint
GO_ENV = MISE_CACHE_DIR=$(MISE_CACHE_DIR) GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)
LINT_ENV = $(GO_ENV) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE)

build:
	$(GO_ENV) go build $(LDFLAGS) -o orc ./cmd/orc/...

install:
	$(GO_ENV) go install $(LDFLAGS) ./cmd/orc/...

clean:
	rm -f orc

test:
	$(GO_ENV) go test ./...

lint:
	$(LINT_ENV) golangci-lint run ./...

tidy:
	$(GO_ENV) go mod tidy

fmt:
	gofmt -l -w ./cmd ./internal

check: lint test

release-check: build
	ORC_BIN="$(CURDIR)/orc" ./scripts/release-check
