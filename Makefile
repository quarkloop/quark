BINARY_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/quarkloop/cli/pkg/config.Version=$(VERSION)

MODULES := \
	core \
	agent \
	agent-api \
	agent-client \
	cli \
	plugins/tool-bash \
	plugins/tool-read \
	plugins/tool-write \
	plugins/tool-web-search

BINARIES := \
	agent/cmd/agent \
	cli/cmd/quark \
	plugins/tool-bash/cmd/bash \
	plugins/tool-read/cmd/read \
	plugins/tool-write/cmd/write \
	plugins/tool-web-search/cmd/web-search

.PHONY: all build clean test test-e2e vet fmt fmt-check tidy \
	build-agent build-cli \
	build-tool-bash build-tool-read build-tool-write build-tool-web-search

all: build

## Build all binaries
build: build-agent build-cli build-tool-bash build-tool-read build-tool-write build-tool-web-search

build-agent:
	go build -o $(BINARY_DIR)/agent ./agent/cmd/agent

build-cli:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/quark ./cli/cmd/quark

build-tool-bash:
	go build -o $(BINARY_DIR)/bash ./plugins/tool-bash/cmd/bash

build-tool-read:
	go build -o $(BINARY_DIR)/read ./plugins/tool-read/cmd/read

build-tool-write:
	go build -o $(BINARY_DIR)/write ./plugins/tool-write/cmd/write

build-tool-web-search:
	go build -o $(BINARY_DIR)/web-search ./plugins/tool-web-search/cmd/web-search

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

## Run gofmt across all modules
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

## Run staticcheck across all modules
lint:
	@issues=0; for mod in $(MODULES); do \
		echo "--- Linting $$mod ---"; \
		if out=$$(cd $$mod && staticcheck ./... 2>&1 | grep -v "^-"); then \
			echo "$$out"; \
			issues=1; \
		fi; \
	done; exit $$issues

## Remove built binaries
clean:
	rm -rf $(BINARY_DIR)

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

build-agent build-cli build-tool-bash build-tool-read build-tool-write build-tool-web-search: | $(BINARY_DIR)

## Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
