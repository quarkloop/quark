BINARY_DIR := bin

MODULES := \
	core \
	agent \
	api-server \
	cli \
	tools/bash \
	tools/kb \
	tools/space \
	tools/web-search

BINARIES := \
	agent/cmd/agent \
	api-server/cmd/api-server \
	cli/cmd/quark \
	tools/bash/cmd/bash \
	tools/kb/cmd/kb \
	tools/space/cmd/space \
	tools/web-search/cmd/web-search

.PHONY: all build clean test vet fmt tidy \
	build-agent build-api-server build-cli \
	build-tools-bash build-tools-kb build-tools-space build-tools-web-search

all: build

## Build all binaries
build: build-agent build-api-server build-cli build-tools-bash build-tools-kb build-tools-space build-tools-web-search

build-agent:
	go build -o $(BINARY_DIR)/agent ./agent/cmd/agent

build-api-server:
	go build -o $(BINARY_DIR)/api-server ./api-server/cmd/api-server

build-cli:
	go build -o $(BINARY_DIR)/quark ./cli/cmd/quark

build-tools-bash:
	go build -o $(BINARY_DIR)/bash ./tools/bash/cmd/bash

build-tools-kb:
	go build -o $(BINARY_DIR)/kb ./tools/kb/cmd/kb

build-tools-space:
	go build -o $(BINARY_DIR)/space ./tools/space/cmd/space

build-tools-web-search:
	go build -o $(BINARY_DIR)/web-search ./tools/web-search/cmd/web-search

## Run tests across all modules
test:
	@for mod in $(MODULES); do \
		echo "--- Testing $$mod ---"; \
		(cd $$mod && go test ./...); \
	done

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

build-agent build-api-server build-cli build-tools-bash build-tools-kb build-tools-space build-tools-web-search: | $(BINARY_DIR)

## Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
	@echo ""
	@echo "Individual build targets:"
	@echo "  build-agent, build-api-server, build-cli,"
	@echo "  build-tools-bash, build-tools-kb, build-tools-space, build-tools-web-search"
