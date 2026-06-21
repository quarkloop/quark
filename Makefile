# =============================================================================
# Quark Platform — Makefile
# =============================================================================
# Common commands for both the Java server and the Go CLI.
#
# No hardcoded paths. All tools (`mvnw`, `go`, `java`) must be on the user's
# PATH. Override them via environment variables if needed:
#
#   make build MAVEN=./mvnw GO=go
#
# Usage:
#   make help              Show all available targets
#   make clean             Remove all build artifacts (Java + Go + state)
#   make build             Build everything (Java jar + Go binary)
#   make test              Run all tests (Java + Go)
#   make verify            Clean + build + test (CI-friendly)
#   make run-example       Run the multi-tenant streaming example (10s)
#   make cli               Build just the Go CLI binary
#   make server-dev        Start Quarkus dev mode (port 8080)
# =============================================================================

# ----- Tool overrides (user can override via env or make CLI) -----
MAVEN       ?= ./mvnw
GO          ?= go
JAVA        ?= java

# ----- Maven options -----
MAVEN_OPTS  ?= -B -q

# ----- Go options -----
# -buildvcs=false disables Go 1.18+ automatic VCS stamping, which fails
# with "error obtaining VCS status: exit status 128" when the .git
# directory is unreadable (CI, sandboxed builds, Docker with mismatched
# ownership). We use -trimpath for reproducibility instead.
GOFLAGS     ?= -trimpath -buildvcs=false

# ----- Project paths (relative — no absolute paths) -----
CLI_DIR          := cli
CLI_BIN          := $(CLI_DIR)/quarkctl
EXAMPLE_DURATION ?= 15
STATE_DIR        := quark-state

# ----- Color output (only in interactive terminals) -----
ifneq (,$(filter $(TERM),xterm xterm-256color screen-256color))
	C_RESET := \033[0m
	C_BOLD  := \033[1m
	C_GREEN := \033[32m
	C_BLUE  := \033[34m
else
	C_RESET :=
	C_BOLD  :=
	C_GREEN :=
	C_BLUE  :=
endif

# =============================================================================
# Help
# =============================================================================

.PHONY: help
help: ## Show this help
	@printf "$(C_BOLD)Quark Platform — Makefile targets$(C_RESET)\n\n"
	@printf "$(C_BOLD)Build & clean$(C_RESET)\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  $(C_GREEN)%-22s$(C_RESET) %s\n", $$1, $$2}'
	@printf "\n$(C_BOLD)Notes$(C_RESET)\n"
	@printf "  - Java targets use $(C_BLUE)\$$(MAVEN)$(C_RESET) (default: ./mvnw)\n"
	@printf "  - Go targets use $(C_BLUE)\$$(GO)$(C_RESET) (default: go)\n"
	@printf "  - Override example duration: $(C_BLUE)make run-example EXAMPLE_DURATION=30$(C_RESET)\n"

# =============================================================================
# Clean
# =============================================================================

.PHONY: clean clean-java clean-go clean-state
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
# Build
# =============================================================================

.PHONY: build build-java build-go
build: build-java build-go ## Build everything (Java jar + Go binary)

build-java: ## Build all Java modules (skip tests)
	@printf "$(C_BLUE)> Building Java modules (mvn install -DskipTests)...$(C_RESET)\n"
	@$(MAVEN) $(MAVEN_OPTS) clean install -DskipTests
	@printf "$(C_GREEN)✓ Java build complete$(C_RESET)\n"

build-go: ## Build the Go CLI binary
	@printf "$(C_BLUE)> Building Go CLI binary...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) build $(GOFLAGS) -o quarkctl .
	@printf "$(C_GREEN)✓ Go build complete: $(CLI_BIN)$(C_RESET)\n"

# =============================================================================
# Test
# =============================================================================

.PHONY: test test-java test-go
test: test-java test-go ## Run all tests (Java + Go)

test-java: ## Run all Java tests (mvn verify)
	@printf "$(C_BLUE)> Running Java tests (mvn verify)...$(C_RESET)\n"
	@$(MAVEN) $(MAVEN_OPTS) verify
	@printf "$(C_GREEN)✓ Java tests passed$(C_RESET)\n"

test-go: ## Run Go tests (go vet + go test)
	@printf "$(C_BLUE)> Running Go tests...$(C_RESET)\n"
	@cd $(CLI_DIR) && $(GO) vet ./... && $(GO) test ./...
	@printf "$(C_GREEN)✓ Go tests passed$(C_RESET)\n"

# =============================================================================
# Combined verify (CI-friendly)
# =============================================================================

.PHONY: verify
verify: clean build test ## Clean → Build → Test (CI-friendly)

# =============================================================================
# Run
# =============================================================================

.PHONY: server-dev run-example run-server cli
server-dev: ## Start Quarkus dev mode (port 8080, hot reload)
	@printf "$(C_BLUE)> Starting Quarkus dev mode (Ctrl+C to stop)...$(C_RESET)\n"
	@cd quark-server && ../$(MAVEN) quarkus:dev

run-server: build-java ## Start the production server (port 8080)
	@printf "$(C_BLUE)> Starting Quark server (Ctrl+C to stop)...$(C_RESET)\n"
	@java -jar quark-server/target/quarkus-app/quarkus-run.jar

run-example: build ## Deploy and observe the streaming example via CLI→Server workflow
	@printf "$(C_BLUE)> Running streaming example ($(EXAMPLE_DURATION)s)...$(C_RESET)\n"
	@./scripts/run-example.sh $(EXAMPLE_DURATION)

cli: $(CLI_BIN) ## Build just the Go CLI binary (alias for build-go)

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
# Install (optional — copies binaries to /usr/local/bin)
# =============================================================================

.PHONY: install-cli
install-cli: $(CLI_BIN) ## Install the CLI binary to /usr/local/bin (requires sudo)
	@printf "$(C_BLUE)> Installing quarkctl to /usr/local/bin...$(C_RESET)\n"
	@cp $(CLI_BIN) /usr/local/bin/quarkctl
	@chmod +x /usr/local/bin/quarkctl
	@printf "$(C_GREEN)✓ Installed: $(C_BOLD)quarkctl$(C_RESET)$(C_GREEN)$(C_RESET)\n"

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
# Docker (build in a clean container — verifies no host dependencies)
# =============================================================================

.PHONY: docker-build-java docker-build-go docker-verify
docker-build-java: ## Build Java project in a clean Docker container (verifies no host deps)
	@printf "$(C_BLUE)> Building Java in Docker (maven:3.9-eclipse-temurin-21)...$(C_RESET)\n"
	docker run --rm -v "$$PWD":/app -w /app maven:3.9-eclipse-temurin-21 \
		mvn -B clean install -DskipTests
	@printf "$(C_GREEN)✓ Java Docker build complete$(C_RESET)\n"

docker-build-go: ## Build Go CLI in a clean Docker container (verifies no host deps)
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
all: build ## Alias for build
