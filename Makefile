BINARY_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/quarkloop/cli/pkg/cli/config.Version=$(VERSION)

MODULES := \
	core \
	agent \
	agent-api \
	agent-client \
	api-server \
	cli \
	tools/bash \
	tools/kb \
	tools/read \
	tools/space \
	tools/write \
	tools/web-search

BINARIES := \
	agent/cmd/agent \
	api-server/cmd/api-server \
	cli/cmd/quark \
	tools/bash/cmd/bash \
	tools/kb/cmd/kb \
	tools/read/cmd/read \
	tools/space/cmd/space \
	tools/write/cmd/write \
	tools/web-search/cmd/web-search

.PHONY: all build clean test test-e2e vet fmt fmt-check tidy \
	build-agent build-api-server build-cli \
	build-tools-bash build-tools-kb build-tools-read build-tools-space build-tools-write build-tools-web-search

all: build

## Build all binaries
build: build-agent build-api-server build-cli build-tools-bash build-tools-kb build-tools-read build-tools-space build-tools-write build-tools-web-search

build-agent:
	go build -o $(BINARY_DIR)/agent ./agent/cmd/agent

build-api-server:
	go build -o $(BINARY_DIR)/api-server ./api-server/cmd/api-server

build-cli:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/quark ./cli/cmd/quark

build-tools-bash:
	go build -o $(BINARY_DIR)/bash ./tools/bash/cmd/bash

build-tools-kb:
	go build -o $(BINARY_DIR)/kb ./tools/kb/cmd/kb

build-tools-read:
	go build -o $(BINARY_DIR)/read ./tools/read/cmd/read

build-tools-space:
	go build -o $(BINARY_DIR)/space ./tools/space/cmd/space

build-tools-write:
	go build -o $(BINARY_DIR)/write ./tools/write/cmd/write

build-tools-web-search:
	go build -o $(BINARY_DIR)/web-search ./tools/web-search/cmd/web-search

## Run tests across all modules
test:
	@for mod in $(MODULES); do \
		echo "--- Testing $$mod ---"; \
		(cd $$mod && go test ./...); \
	done

## Run E2E tests (requires OPENROUTER_API_KEY or ZHIPU_API_KEY; loads quark/.env when present)
test-e2e:
	go test -tags e2e -v -timeout 10m ./agent/e2e

## Run vet across all modules
vet:
	@for mod in $(MODULES); do \
		echo "--- Vetting $$mod ---"; \
		(cd $$mod && go vet ./...); \
	done

## Run gofmt check across all modules
fmt:
	@for mod in $(MODULES); do \
		echo "--- Formatting $$mod ---"; \
		(cd $$mod && gofmt -w .); \
	done

## Check formatting without modifying files (exits non-zero if any file is unformatted)
fmt-check:
	@unformatted=$$(for mod in $(MODULES); do (cd $$mod && gofmt -l .); done); \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files:"; echo "$$unformatted"; exit 1; \
	fi

## Run go mod tidy across all modules
tidy:
	@for mod in $(MODULES); do \
		echo "--- Tidying $$mod ---"; \
		(cd $$mod && go mod tidy); \
	done

## Remove built binaries
clean:
	rm -rf $(BINARY_DIR)

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

build-agent build-api-server build-cli build-tools-bash build-tools-kb build-tools-read build-tools-space build-tools-write build-tools-web-search: | $(BINARY_DIR)

## Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
	@echo ""
	@echo "Individual build targets:"
	@echo "  build-agent, build-api-server, build-cli,"
	@echo "  build-tools-bash, build-tools-kb, build-tools-read,"
	@echo "  build-tools-space, build-tools-write, build-tools-web-search"
