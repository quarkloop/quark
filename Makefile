BINARY_DIR := bin

MODULES := \
	agent \
	space \
	api-server \
	kb \
	cli \
	tools/bash \
	tools/web-search

BINARIES := \
	agent/cmd/agent \
	space/cmd/space \
	api-server/cmd/api-server \
	kb/cmd/kb \
	cli/cmd/quark \
	tools/bash/cmd/bash \
	tools/web-search/cmd/web-search

.PHONY: all build clean test vet fmt tidy \
	build-agent build-space build-api-server build-kb build-cli \
	build-tools-bash build-tools-web-search

all: build

## Build all binaries
build: $(addprefix build-,$(subst /,-,$(MODULES)))

build-agent:
	go build -o $(BINARY_DIR)/agent ./agent/cmd/agent

build-space:
	go build -o $(BINARY_DIR)/space ./space/cmd/space

build-api-server:
	go build -o $(BINARY_DIR)/api-server ./api-server/cmd/api-server

build-kb:
	go build -o $(BINARY_DIR)/kb ./kb/cmd/kb

build-cli:
	go build -o $(BINARY_DIR)/quark ./cli/cmd/quark

build-tools-bash:
	go build -o $(BINARY_DIR)/bash ./tools/bash/cmd/bash

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
	@echo "--- Vetting store ---"
	@(cd store && go vet ./...)

## Run gofmt check across all modules
fmt:
	@for mod in $(MODULES) store; do \
		echo "--- Formatting $$mod ---"; \
		(cd $$mod && gofmt -w .); \
	done

## Run go mod tidy across all modules
tidy:
	@for mod in $(MODULES) store; do \
		echo "--- Tidying $$mod ---"; \
		(cd $$mod && go mod tidy); \
	done

## Remove built binaries
clean:
	rm -rf $(BINARY_DIR)

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

build-agent build-space build-api-server build-kb build-cli build-tools-bash build-tools-web-search: | $(BINARY_DIR)

## Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
	@echo ""
	@echo "Individual build targets:"
	@echo "  build-agent, build-space, build-api-server, build-kb,"
	@echo "  build-cli, build-tools-bash, build-tools-web-search"
