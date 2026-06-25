# =============================================================================
# Quark Platform — Makefile
# =============================================================================
# Build system for the v6 Quark Platform:
#   server/    — control plane (Go + Fiber, single binary)
#   runtime/   — data plane (Java/GraalJS, with GraalJS/Truffle)
#   quark-catalog/ — Catalog service (Go + SQLite)
#   cli/       — quarkctl (Go + Cobra)
#
# The control plane is now a single Go binary. It spawns the data plane
# (Java runtime with GraalJS) as a child process via ProcessManager.
#
# Quick start:
#   make build                  # Go server + Go CLI + Go Catalog + Java runtime
#   make run-example            # Run example with JVM runtime
#
#   make build-native-runtime   # Native data plane with GraalJS (~9 min, 6.5 GB RAM)
#   make run-example RUN_MODE=native  # Same but with native runtime binary
#
# Native builds require Oracle GraalVM 21+ with native-image on PATH
# (or GRAALVM_HOME set). The server is always a Go binary (no native-image).
# =============================================================================

# ----- Tool overrides (user can override via env or make CLI) -----
MAVEN       ?= ./mvnw
GO          ?= go
JAVA        ?= java

# ----- Maven options -----
MAVEN_OPTS  ?= -B -q

# If GRAALVM_HOME is set, use its java and native-image
ifneq ($(GRAALVM_HOME),)
JAVA := $(GRAALVM_HOME)/bin/java
PATH := $(GRAALVM_HOME)/bin:$(PATH)
endif

# ----- Go options -----
GOFLAGS     ?= -trimpath -buildvcs=false

# ----- Project paths -----
CLI_DIR          := cli
CLI_BIN          := $(CLI_DIR)/quarkctl
SERVER_DIR       := server
SERVER_BIN       := $(SERVER_DIR)/quark-server
CATALOG_DIR      := quark-catalog
CATALOG_BIN      := $(CATALOG_DIR)/quark-catalog
EXAMPLE_DURATION ?= 15
STATE_DIR        := quark-state

# ----- Binary paths -----
# Control plane (server) — Go binary (single binary, no JVM/native split)
SERVER_GO_MAIN := $(SERVER_DIR)/cmd/server

# Data plane (runtime) — JVM and native variants
# Note: Quarkus names native output as <finalName>-runner, and we set
# finalName=quark-runtime-runner, so the binary is quark-runtime-runner-runner
# (yes, double -runner suffix). The JVM runnable jar follows the same rule:
# <finalName>-runner.jar = quark-runtime-runner-runner.jar.
# The thin jar <finalName>.jar = quark-runtime-runner.jar has no main manifest
# and is NOT runnable — do not use it for `java -jar`.
RUNTIME_JAR    := runtime/quark-runtime/target/quark-runtime-runner-runner.jar
RUNTIME_NATIVE := runtime/quark-runtime/target/quark-runtime-runner-runner

# Default run mode = JVM (use RUN_MODE=native for native runtime)
# Note: RUN_MODE only affects the runtime now — the server is always Go.
RUN_MODE ?= jvm

# ----- Color output -----
ifneq (,$(filter $(TERM),xterm xterm-256color screen-256color))
        C_RESET := \033[0m
        C_BOLD  := \033[1m
        C_GREEN := \033[32m
        C_BLUE  := \033[34m
        C_YELLOW:= \033[33m
        C_RED   := \033[31m
else
        C_RESET :=
        C_BOLD  :=
        C_GREEN :=
        C_BLUE  :=
        C_YELLOW:=
        C_RED   :=
endif

# Mode label for log messages
# (RUN_MODE only affects the runtime; the server is always Go.)
ifeq ($(RUN_MODE),native)
MODE_LABEL := $(C_YELLOW)[native-runtime]$(C_RESET)
else
MODE_LABEL := $(C_BLUE)[jvm-runtime]$(C_RESET)
endif

# =============================================================================
# Help
# =============================================================================

.PHONY: help
help: ## Show this help
	@printf "$(C_BOLD)Quark Platform — Makefile targets$(C_RESET)\n\n"
	@printf "$(C_BOLD)Run mode:$(C_RESET) $(MODE_LABEL)  (set RUN_MODE=native for native binaries)\n\n"
	@printf "$(C_BOLD)Build & clean$(C_RESET)\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
                | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(C_GREEN)%-26s$(C_RESET) %s\n", $$1, $$2}'
	@printf "\n$(C_BOLD)Examples$(C_RESET)\n"
	@printf "  $(C_BLUE)make build$(C_RESET)                          # JVM build (all modules + CLI + Catalog)\n"
	@printf "  $(C_BLUE)make build-native$(C_RESET)                   # Both native binaries (~13 min total)\n"
	@printf "  $(C_BLUE)make run-example$(C_RESET)                    # Run example (JVM)\n"
	@printf "  $(C_BLUE)make run-example RUN_MODE=native$(C_RESET)    # Run example (native)\n"

# =============================================================================
# Clean
# =============================================================================

.PHONY: clean clean-java clean-go clean-state clean-native
clean: clean-java clean-go clean-state ## Remove all build artifacts (Java + Go + state)

clean-java: ## Remove Java target/ directories
	@printf "$(C_BLUE)> Cleaning Java target directories...$(C_RESET)\n"
	@find . -name target -type d -prune -exec rm -rf {} + 2>/dev/null || true
	@printf "$(C_GREEN)✓ Java cleaned$(C_RESET)\n"

clean-go: ## Remove Go build artifacts
	@printf "$(C_BLUE)> Cleaning Go build artifacts...$(C_RESET)\n"
	@rm -f $(CLI_BIN) $(CATALOG_BIN) $(SERVER_BIN)
	@printf "$(C_GREEN)✓ Go cleaned$(C_RESET)\n"

clean-state: ## Remove persisted platform state
	@printf "$(C_BLUE)> Removing platform state...$(C_RESET)\n"
	@rm -rf $(STATE_DIR)
	@rm -f example/simple-streaming/json/system-monitor.jsonl
	@printf "$(C_GREEN)✓ State cleaned$(C_RESET)\n"

clean-native: ## Remove native binary artifacts only
	@printf "$(C_BLUE)> Cleaning native binaries...$(C_RESET)\n"
	@rm -f $(RUNTIME_NATIVE)
	@printf "$(C_GREEN)✓ Native binaries cleaned$(C_RESET)\n"

# =============================================================================
# Build — JVM mode (default)
# =============================================================================

.PHONY: build build-java build-go build-catalog build-server

build: build-java build-go build-catalog ## Build everything (Java runtime + Go server + Go CLI + Catalog)

build-java: ## Build all Java modules (JVM mode, skip tests)
	@printf "$(C_BLUE)$(MODE_LABEL) > Building Java runtime modules (JVM mode)...$(C_RESET)\n"
	@$(MAVEN) $(MAVEN_OPTS) clean install -DskipTests
	@printf "$(C_GREEN)✓ Java build complete$(C_RESET)\n"
	@printf "    Runtime JAR:   $(RUNTIME_JAR)\n"

build-go: build-server ## Build the Go CLI + Go server binaries
	@printf "$(C_BLUE)> Building Go CLI binary...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) build $(GOFLAGS) -o quarkctl .
	@printf "$(C_GREEN)✓ Go CLI build complete: $(CLI_BIN)$(C_RESET)\n"

build-server: ## Build the Go control plane server binary
	@printf "$(C_BLUE)> Building Go server binary...$(C_RESET)\n"
	@cd $(SERVER_DIR) && $(GO) build $(GOFLAGS) -o quark-server ./cmd/server
	@printf "$(C_GREEN)✓ Go server build complete: $(SERVER_BIN)$(C_RESET)\n"

build-catalog: ## Build the Catalog service (Go + SQLite)
	@printf "$(C_BLUE)> Building Catalog service...$(C_RESET)\n"
	@cd $(CATALOG_DIR) && $(GO) build $(GOFLAGS) -o quark-catalog ./cmd/quark-catalog
	@printf "$(C_GREEN)✓ Catalog build complete: $(CATALOG_BIN)$(C_RESET)\n"

# =============================================================================
# Build — Native mode (runtime only; server is Go, no native-image)
# =============================================================================

# Check that native-image is available
define check_native_image
	@command -v native-image >/dev/null 2>&1 || { \
                printf "$(C_RED)✗ native-image not found. Install Oracle GraalVM 21+ and ensure native-image is on PATH$(C_RESET)\n"; \
                printf "    Or set GRAALVM_HOME=/path/to/graalvm-jdk-21\n"; \
                exit 1; \
        }
endef

.PHONY: build-native build-native-runtime

build-native: build-native-runtime ## Build the native runtime binary (server is Go, always built by build-go)

build-native-runtime: ## Build the data plane native binary with GraalJS (~9 min, 6.5 GB RAM, 194 MB output)
	@printf "$(C_BLUE)[native] > Building data plane (runtime) native image with GraalJS/Truffle...$(C_RESET)\n"
	$(check_native_image)
	@$(MAVEN) $(MAVEN_OPTS) -pl runtime/quark-runtime -am -Pnative install -DskipTests
	@printf "$(C_GREEN)✓ Runtime native build complete$(C_RESET)\n"
	@ls -lh $(RUNTIME_NATIVE)

# Legacy alias — kept for backwards compat with older docs
build-mode-native: build-native ## Alias for build-native

# =============================================================================
# Test
# =============================================================================

.PHONY: test test-java test-go
test: test-java test-go ## Run all tests (Java + Go, JVM mode)

test-java: ## Run all Java tests (mvn verify, JVM mode)
	@printf "$(C_BLUE)$(MODE_LABEL) > Running Java tests...$(C_RESET)\n"
	@$(MAVEN) $(MAVEN_OPTS) verify
	@printf "$(C_GREEN)✓ Java tests passed$(C_RESET)\n"

test-go: ## Run Go tests (go vet + go test for CLI, server, catalog)
	@printf "$(C_BLUE)> Running Go tests...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) vet ./... && $(GO) test ./...
	@cd $(SERVER_DIR) && $(GO) vet ./... && $(GO) test ./...
	@cd $(CATALOG_DIR) && $(GO) vet ./... && $(GO) test ./...
	@printf "$(C_GREEN)✓ Go tests passed$(C_RESET)\n"

# =============================================================================
# Verify (CI-friendly)
# =============================================================================

.PHONY: verify verify-native
verify: clean build test ## Clean → Build → Test (JVM mode, CI-friendly)

verify-native: clean-native build-native ## Clean → Build native runtime binary
	@printf "$(C_GREEN)✓ Native verify complete (runtime binary built)$(C_RESET)\n"

# =============================================================================
# Run
# =============================================================================

.PHONY: server-dev run-server run-example cli

server-dev: build-server ## Start the Go server in dev mode (auto-rebuild on file changes)
	@printf "$(C_BLUE)> Starting Go server (Ctrl+C to stop)...$(C_RESET)\n"
	@cd $(SERVER_DIR) && $(GO) run ./cmd/server

run-server: build-server ## Start the Go control plane server (port 8080)
	@printf "$(C_BLUE)$(MODE_LABEL) > Starting Quark Go server...$(C_RESET)\n"
	@$(SERVER_BIN)

run-example: ## Deploy and observe the streaming example (mode-dependent)
	@printf "$(C_BLUE)$(MODE_LABEL) > Running streaming example ($(EXAMPLE_DURATION)s)...$(C_RESET)\n"
ifeq ($(RUN_MODE),native)
	@$(MAKE) build-native-runtime
	@$(MAKE) build-go
	@$(MAKE) build-catalog
	@RUN_MODE=native ./scripts/run-example.sh $(EXAMPLE_DURATION)
else
	@$(MAKE) build
	@RUN_MODE=jvm ./scripts/run-example.sh $(EXAMPLE_DURATION)
endif

cli: $(CLI_BIN) ## Build just the Go CLI binary

$(CLI_BIN):
	@cd $(CLI_DIR) && $(GO) build $(GOFLAGS) -o quarkctl .

# =============================================================================
# Format / lint
# =============================================================================

.PHONY: fmt lint
fmt: ## Format Go code (gofmt)
	@printf "$(C_BLUE)> Formatting Go code...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) fmt ./...
	@cd $(SERVER_DIR) && $(GO) fmt ./...
	@cd $(CATALOG_DIR) && $(GO) fmt ./...
	@printf "$(C_GREEN)✓ Go formatted$(C_RESET)\n"

lint: ## Run linters (go vet for Go)
	@printf "$(C_BLUE)> Linting Go code (go vet)...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) vet ./...
	@cd $(SERVER_DIR) && $(GO) vet ./...
	@cd $(CATALOG_DIR) && $(GO) vet ./...
	@printf "$(C_GREEN)✓ Go lint clean$(C_RESET)\n"

# =============================================================================
# Distribution
# =============================================================================

.PHONY: dist
dist: build-go ## Build platform-specific CLI binaries into dist/
	@printf "$(C_BLUE)> Building platform-specific CLI binaries...$(C_RESET)\n"
	@mkdir -p dist
	@cd $(CLI_DIR) && \
	GOOS=darwin  GOARCH=arm64 $(GO) build $(GOFLAGS) -o ../dist/quarkctl-darwin-arm64  . && \
	GOOS=darwin  GOARCH=amd64 $(GO) build $(GOFLAGS) -o ../dist/quarkctl-darwin-amd64  . && \
	GOOS=linux   GOARCH=amd64 $(GO) build $(GOFLAGS) -o ../dist/quarkctl-linux-amd64   . && \
	GOOS=linux   GOARCH=arm64 $(GO) build $(GOFLAGS) -o ../dist/quarkctl-linux-arm64   . && \
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o ../dist/quarkctl-windows-amd64.exe .
	@printf "$(C_GREEN)✓ Built 5 binaries in dist/$(C_RESET)\n"
	@ls -lh dist/

# =============================================================================
# Docker
# =============================================================================

.PHONY: docker-build-java docker-build-go docker-verify docker-build-native
docker-build-java: ## Build Java project in a clean Docker container
	@printf "$(C_BLUE)> Building Java in Docker (maven:3.9-eclipse-temurin-21)...$(C_RESET)\n"
	docker run --rm -v "$$PWD":/app -w /app maven:3.9-eclipse-temurin-21 \
	mvn -B clean install -DskipTests
	@printf "$(C_GREEN)✓ Java Docker build complete$(C_RESET)\n"

docker-build-native: ## Build the native runtime in Docker (Mandrel builder image)
	@printf "$(C_BLUE)> Building native runtime in Docker (quay.io/quarkus/ubi-quarkus-mandrel-builder-image)...$(C_RESET)\n"
	docker run --rm -v "$$PWD":/app -w /app maven:3.9-eclipse-temurin-21 \
	mvn -B -pl runtime/quark-runtime -am -Pnative clean install -DskipTests
	@printf "$(C_GREEN)✓ Native Docker build complete (runtime binary)$(C_RESET)\n"

docker-build-go: ## Build Go CLI in a clean Docker container
	@printf "$(C_BLUE)> Building Go in Docker (golang:1.24)...$(C_RESET)\n"
	docker run --rm -v "$$PWD":/app -w /app/cli golang:1.24 \
	go build -trimpath -buildvcs=false -o /app/$(CLI_BIN) .
	@printf "$(C_GREEN)✓ Go Docker build complete: $(CLI_BIN)$(C_RESET)\n"

docker-verify: ## Full clean build + test in Docker (CI-friendly, no host deps)
	@printf "$(C_BLUE)> Full verify in Docker...$(C_RESET)\n"
	docker run --rm -v "$$PWD":/app -w /app maven:3.9-eclipse-temurin-21 \
	mvn -B clean verify
	docker run --rm -v "$$PWD":/app -w /app/cli golang:1.24 \
	sh -c 'go vet ./... && go test ./... && go build -trimpath -buildvcs=false -o /app/$(CLI_BIN) .'
	@printf "$(C_GREEN)✓ Docker verify complete$(C_RESET)\n"

# =============================================================================
# Aliases
# =============================================================================

.PHONY: all
all: build ## Alias for build (JVM mode)
