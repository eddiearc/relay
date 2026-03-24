.DEFAULT_GOAL := build

APP := relay
CMD := ./cmd/relay
BIN_DIR := bin
DIST_DIR := dist

GO ?= go
HOST_GOOS := $(shell if command -v $(GO) >/dev/null 2>&1; then $(GO) env GOOS; else uname -s | tr '[:upper:]' '[:lower:]'; fi)
HOST_GOARCH := $(shell if command -v $(GO) >/dev/null 2>&1; then $(GO) env GOARCH; else uname -m | sed -e 's/^x86_64$$/amd64/' -e 's/^aarch64$$/arm64/' -e 's/^arm64$$/arm64/'; fi)
GOOS ?= $(HOST_GOOS)
GOARCH ?= $(HOST_GOARCH)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
	-X github.com/eddiearc/relay/internal/cli.version=$(VERSION) \
	-X github.com/eddiearc/relay/internal/cli.commit=$(COMMIT) \
	-X github.com/eddiearc/relay/internal/cli.buildDate=$(BUILD_DATE)

PACKAGE_NAME := $(APP)_$(VERSION)_$(GOOS)_$(GOARCH)
PACKAGE_DIR := $(DIST_DIR)/$(PACKAGE_NAME)

.PHONY: help test build package package-all clean

help:
	@echo "make build        Build the current platform binary into $(BIN_DIR)/$(APP)"
	@echo "make package      Build and archive the current platform package into $(DIST_DIR)/"
	@echo "make package-all  Build and archive linux/darwin release packages"
	@echo "make test         Run Go tests"
	@echo "make clean        Remove $(BIN_DIR)/ and $(DIST_DIR)/"
	@echo ""
	@echo "Override variables as needed, for example:"
	@echo "  make package VERSION=v0.1.0"
	@echo "  make build GOOS=linux GOARCH=arm64"

test:
	$(GO) test ./...

build:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP) $(CMD)

package:
	mkdir -p $(PACKAGE_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(PACKAGE_DIR)/$(APP) $(CMD)
	cp README.md $(PACKAGE_DIR)/README.md
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(PACKAGE_NAME).tar.gz $(PACKAGE_NAME)

package-all:
	@set -e; \
	for target in "linux amd64" "linux arm64" "darwin amd64" "darwin arm64"; do \
		set -- $$target; \
		$(MAKE) package GOOS=$$1 GOARCH=$$2 VERSION=$(VERSION); \
	done

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
