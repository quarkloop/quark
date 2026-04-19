BINARY_DIR := bin
PLUGIN_DIR := plugins
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/quarkloop/cli/pkg/buildinfo.Version=$(VERSION)

# Tool plugins
TOOLS := bash read write web-search

# Provider plugins
PROVIDERS := openrouter openai anthropic

# All modules for testing/vetting
MODULES := \
	supervisor \
	agent \
	cli \
	pkg/plugin \
	plugins/tools/bash \
	plugins/tools/read \
	plugins/tools/write \
	plugins/tools/web-search \
	plugins/providers/openrouter \
	plugins/providers/openai \
	plugins/providers/anthropic

.PHONY: all build clean test test-e2e vet fmt fmt-check tidy \
	build-supervisor build-agent build-cli \
	build-plugins build-tools build-tools-lib build-providers

all: build

## Build all binaries
build: build-supervisor build-agent build-cli build-tools

## Build all plugins (tools as binary + lib, providers as lib)
build-plugins: build-tools build-tools-lib build-providers

## Build tool plugins as binaries
build-tools:
	@for tool in $(TOOLS); do \
		echo "--- Building tool (binary): $$tool ---"; \
		go build -o $(BINARY_DIR)/$$tool ./$(PLUGIN_DIR)/tools/$$tool/cmd/$$tool; \
	done

## Build tool plugins as .so files (lib mode, requires CGO)
build-tools-lib:
	@for tool in $(TOOLS); do \
		if [ -f $(PLUGIN_DIR)/tools/$$tool/plugin.go ]; then \
			echo "--- Building tool (lib): $$tool ---"; \
			CGO_ENABLED=1 go build -buildmode=plugin -tags plugin \
				-o $(PLUGIN_DIR)/tools/$$tool/plugin.so \
				./$(PLUGIN_DIR)/tools/$$tool; \
		fi; \
	done

## Build provider plugins as .so files (requires CGO)
build-providers:
	@for provider in $(PROVIDERS); do \
		echo "--- Building provider: $$provider ---"; \
		CGO_ENABLED=1 go build -buildmode=plugin -tags plugin \
			-o $(PLUGIN_DIR)/providers/$$provider/plugin.so \
			./$(PLUGIN_DIR)/providers/$$provider; \
	done

build-supervisor:
	go build -o $(BINARY_DIR)/supervisor ./supervisor/cmd/supervisor

build-agent:
	go build -o $(BINARY_DIR)/agent ./agent/cmd/agent

build-cli:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/quark ./cli/cmd/quark

## Run tests across all modules
test:
	@for mod in $(MODULES); do \
		echo "--- Testing $$mod ---"; \
		(cd $$mod && go test ./...); \
	done

## Run E2E tests (requires OPENROUTER_API_KEY or ZHIPU_API_KEY; loads quark/.env when present)
test-e2e:
	go test -tags e2e -v -timeout 10m ./e2e

## Run vet across all modules (providers are vetted under the `plugin` build
## tag since all their sources are gated on it)
vet:
	@set -e; for mod in $(MODULES); do \
		echo "--- Vetting $$mod ---"; \
		case $$mod in \
			plugins/providers/*) (cd $$mod && go vet -tags plugin ./...);; \
			*) (cd $$mod && go vet ./...);; \
		esac; \
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

## Run staticcheck across all modules (providers linted under the `plugin` tag)
lint:
	@issues=0; for mod in $(MODULES); do \
		echo "--- Linting $$mod ---"; \
		case $$mod in \
			plugins/providers/*) out=$$(cd $$mod && staticcheck -tags plugin ./... 2>&1 | grep -v "^-");; \
			*) out=$$(cd $$mod && staticcheck ./... 2>&1 | grep -v "^-");; \
		esac; \
		if [ -n "$$out" ]; then echo "$$out"; issues=1; fi; \
	done; exit $$issues

## Remove built binaries and plugin .so files
clean:
	rm -rf $(BINARY_DIR)
	find $(PLUGIN_DIR) -name "*.so" -delete 2>/dev/null || true

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

build-supervisor build-agent build-cli build-tools: | $(BINARY_DIR)

## Show available targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
