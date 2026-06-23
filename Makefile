# =============================================================================
# Quark Platform — Makefile
# =============================================================================
# Build system supporting BOTH JVM and GraalVM Native Image modes.
#
# Default mode is JVM (java -jar). To build/run in native mode, prefix the
# target with BUILD_MODE=native. The Makefile detects the build mode and
# uses the correct binary for running and spawning data-plane processes.
#
# Quick start:
#   make build              # JVM build (default)
#   make run-example        # Run example with JVM server
#
#   make build BUILD_MODE=native   # Native build
#   make run-example BUILD_MODE=native  # Run example with native server
#
# All targets accept BUILD_MODE=jvm|native. The mode controls:
#   - Which Maven profile is activated (-Pnative for native)
#   - Which binary is run (quark-server-*.jar vs quark-server native exe)
#   - How data-plane processes are spawned (java -jar vs native binary)
# =============================================================================

# ----- Build mode -----
# BUILD_MODE controls whether we build/run JVM or native. Default: jvm
BUILD_MODE  ?= jvm

# Validate BUILD_MODE
ifneq ($(BUILD_MODE),jvm)
ifneq ($(BUILD_MODE),native)
$(error BUILD_MODE must be 'jvm' or 'native', got: $(BUILD_MODE))
endif
endif

# ----- Tool overrides (user can override via env or make CLI) -----
MAVEN       ?= ./mvnw
GO          ?= go
JAVA        ?= java

# ----- Maven options -----
MAVEN_OPTS  ?= -B -q

# If GRAALVM_HOME is set, use its java and native-image
ifneq ($(GRAALVM_HOME),)
JAVA := $(GRAALVM_HOME)/bin/java
endif

# ----- Go options -----
GOFLAGS     ?= -trimpath -buildvcs=false

# ----- Project paths -----
CLI_DIR          := cli
CLI_BIN          := $(CLI_DIR)/quarkctl
EXAMPLE_DURATION ?= 15
STATE_DIR        := quark-state

# ----- Binary paths (mode-dependent) -----
# JVM mode:   quark-server/target/quark-server-0.1.0-SNAPSHOT-runner.jar
# Native mode: quark-server/target/quark-server-0.1.0-SNAPSHOT-runner (no extension)
SERVER_JAR := quark-server/target/quark-server-0.1.0-SNAPSHOT-runner.jar
SERVER_NATIVE := quark-server/target/quark-server-0.1.0-SNAPSHOT-runner
ifeq ($(BUILD_MODE),native)
SERVER_BIN := $(SERVER_NATIVE)
RUN_CMD := $(SERVER_NATIVE)
else
SERVER_BIN := $(SERVER_JAR)
RUN_CMD := $(JAVA) -jar $(SERVER_JAR)
endif

# ----- Color output -----
ifneq (,$(filter $(TERM),xterm xterm-256color screen-256color))
	C_RESET := \033[0m
	C_BOLD  := \033[1m
	C_GREEN := \033[32m
	C_BLUE  := \033[34m
	C_YELLOW:= \033[33m
else
	C_RESET :=
	C_BOLD  :=
	C_GREEN :=
	C_BLUE  :=
	C_YELLOW:=
endif

# Mode label for log messages
ifeq ($(BUILD_MODE),native)
MODE_LABEL := $(C_YELLOW)[native]$(C_RESET)
else
MODE_LABEL := $(C_BLUE)[jvm]$(C_RESET)
endif

# =============================================================================
# Help
# =============================================================================

.PHONY: help
help: ## Show this help
	@printf "$(C_BOLD)Quark Platform — Makefile targets$(C_RESET)\n\n"
	@printf "$(C_BOLD)Build mode:$(C_RESET) $(MODE_LABEL)\n"
	@printf "$(C_BOLD)  BUILD_MODE=jvm      (default) java -jar runner.jar$(C_RESET)\n"
	@printf "$(C_BOLD)  BUILD_MODE=native   GraalVM native executable$(C_RESET)\n\n"
	@printf "$(C_BOLD)Build & clean$(C_RESET)\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	        | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(C_GREEN)%-26s$(C_RESET) %s\n", $$1, $$2}'
	@printf "\n$(C_BOLD)Examples$(C_RESET)\n"
	@printf "  $(C_BLUE)make build$(C_RESET)                          # JVM build\n"
	@printf "  $(C_BLUE)make build BUILD_MODE=native$(C_RESET)       # Native build\n"
	@printf "  $(C_BLUE)make run-example$(C_RESET)                   # Run example (JVM)\n"
	@printf "  $(C_BLUE)make run-example BUILD_MODE=native$(C_RESET) # Run example (native)\n"

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
	@rm -f $(CLI_BIN)
	@printf "$(C_GREEN)✓ Go cleaned$(C_RESET)\n"

clean-state: ## Remove persisted platform state
	@printf "$(C_BLUE)> Removing platform state...$(C_RESET)\n"
	@rm -rf $(STATE_DIR)
	@rm -f example/simple-streaming/json/system-monitor.jsonl
	@printf "$(C_GREEN)✓ State cleaned$(C_RESET)\n"

# =============================================================================
# Build — JVM mode (default)
# =============================================================================

.PHONY: build build-java build-go build-native

build: build-java build-go ## Build everything (JVM jar + Go binary)

build-java: ## Build all Java modules (JVM mode, skip tests)
	@printf "$(C_BLUE)$(MODE_LABEL) > Building Java modules (JVM mode)...$(C_RESET)\n"
	@$(MAVEN) $(MAVEN_OPTS) clean install -DskipTests
	@printf "$(C_GREEN)✓ Java build complete: $(SERVER_JAR)$(C_RESET)\n"

build-go: ## Build the Go CLI binary
	@printf "$(C_BLUE)> Building Go CLI binary...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) build $(GOFLAGS) -o quarkctl .
	@printf "$(C_GREEN)✓ Go build complete: $(CLI_BIN)$(C_RESET)\n"

# =============================================================================
# Build — Native mode
# =============================================================================

build-native: ## Build the native executable (requires GraalVM/Mandrel native-image)
	@printf "$(C_BLUE)$(MODE_LABEL) > Building native executable...$(C_RESET)\n"
	@command -v native-image >/dev/null 2>&1 || { \
	        printf "$(C_RED)✗ native-image not found. Install GraalVM/Mandrel and ensure native-image is on PATH$(C_RESET)\n"; \
	        exit 1; \
	}
	@$(MAVEN) $(MAVEN_OPTS) -Pnative clean install
	@printf "$(C_GREEN)✓ Native build complete: $(SERVER_NATIVE)$(C_RESET)\n"
	@ls -lh $(SERVER_NATIVE)

# Unified build target that respects BUILD_MODE
build-mode: ## Build in the mode specified by BUILD_MODE (jvm or native)
ifeq ($(BUILD_MODE),native)
	$(MAKE) build-native
else
	$(MAKE) build
endif

# =============================================================================
# Test
# =============================================================================

.PHONY: test test-java test-go
test: test-java test-go ## Run all tests (Java + Go, JVM mode)

test-java: ## Run all Java tests (mvn verify, JVM mode)
	@printf "$(C_BLUE)$(MODE_LABEL) > Running Java tests...$(C_RESET)\n"
	@$(MAVEN) $(MAVEN_OPTS) verify
	@printf "$(C_GREEN)✓ Java tests passed$(C_RESET)\n"

test-go: ## Run Go tests (go vet + go test)
	@printf "$(C_BLUE)> Running Go tests...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) vet ./... && $(GO) test ./...
	@printf "$(C_GREEN)✓ Go tests passed$(C_RESET)\n"

# =============================================================================
# Verify (CI-friendly)
# =============================================================================

.PHONY: verify verify-native
verify: clean build test ## Clean → Build → Test (JVM mode, CI-friendly)

verify-native: clean-native build-native ## Clean → Build → Test (native mode)
	@printf "$(C_GREEN)✓ Native verify complete$(C_RESET)\n"

# =============================================================================
# Run
# =============================================================================

.PHONY: server-dev run-server run-example cli

server-dev: ## Start Quarkus dev mode (port 8080, hot reload)
	@printf "$(C_BLUE)> Starting Quarkus dev mode (Ctrl+C to stop)...$(C_RESET)\n"
	@cd quark-server && ../$(MAVEN) quarkus:dev

run-server: build-java ## Start the production server (JVM mode, port 8080)
	@printf "$(C_BLUE)$(MODE_LABEL) > Starting Quark server...$(C_RESET)\n"
	@$(RUN_CMD)

run-server-native: build-native ## Start the production server (native mode)
	@printf "$(C_BLUE)$(MODE_LABEL) > Starting native Quark server...$(C_RESET)\n"
	@$(SERVER_NATIVE)

run-example: ## Deploy and observe the streaming example (mode-dependent)
	@printf "$(C_BLUE)$(MODE_LABEL) > Running streaming example ($(EXAMPLE_DURATION)s)...$(C_RESET)\n"
ifeq ($(BUILD_MODE),native)
	@$(MAKE) build-native
	@BUILD_MODE=native ./scripts/run-example.sh $(EXAMPLE_DURATION)
else
	@$(MAKE) build
	@BUILD_MODE=jvm ./scripts/run-example.sh $(EXAMPLE_DURATION)
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
	@printf "$(C_GREEN)✓ Go formatted$(C_RESET)\n"

lint: ## Run linters (go vet for Go)
	@printf "$(C_BLUE)> Linting Go code (go vet)...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) vet ./...
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

docker-build-native: ## Build native executable in Docker (Mandrel builder image)
	@printf "$(C_BLUE)> Building native in Docker (quay.io/quarkus/ubi-quarkus-mandrel-builder-image)...$(C_RESET)\n"
	docker run --rm -v "$$PWD":/app -w /app maven:3.9-eclipse-temurin-21 \
	        mvn -B -Pnative clean install
	@printf "$(C_GREEN)✓ Native Docker build complete$(C_RESET)\n"

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
