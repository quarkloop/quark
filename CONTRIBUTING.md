# Contributing to Quark

Thanks for your interest in contributing! This document describes how to set up a development environment and submit changes.

## Development setup

### Prerequisites

The Quark platform has three components with different toolchain requirements:

| Component | Language | Toolchain |
|---|---|---|
| Control plane (`quark-server/`) | Go | Go ≥ 1.24 |
| Catalog service (`quark-catalog/`) | Go | Go ≥ 1.24 |
| CLI (`quark-cli/`) | Go | Go ≥ 1.24 |
| Data plane (`quark-runtime/`) | Java | JDK 21 + Maven 3.9+ |
| Native data plane (optional) | Java/GraalVM | GraalVM 21+ with `native-image` |

External runtime dependencies:

- A running [NATS server](https://nats.io/download-nats-io/) for inter-service communication.

### Install dependencies

```bash
git clone https://github.com/quarkloop/quark.git
cd quark

# Go dependencies are fetched automatically on first build.
# Java dependencies are fetched by Maven on first build.
```

### Build

```bash
# Build everything (Go control plane + CLI + Catalog + Java runtime):
make build

# Build only the Go components:
make go

# Build only the Java runtime:
make runtime

# Native-image build of the runtime (requires GraalVM):
make native
```

### Test

```bash
# Go unit tests for all three Go modules:
make test-go

# Java unit tests:
make test-java

# Full E2E example (starts NATS, builds everything, runs simple-streaming):
make run-example
```

### Verify in a clean container

```bash
make docker-verify
```

This builds the entire project inside Docker containers with no host toolchain — useful for confirming the build does not depend on any local installations.

## Submitting changes

### Pull requests

1. Fork the repository and create a feature branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```
2. Make your changes. Keep commits focused — one logical change per commit.
3. Ensure `make test-go` and `make test-java` both pass.
4. Write a clear PR description explaining what changed and why.
5. Reference any related issues (e.g. `Closes #42`).

### Commit message conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix | Use |
|---|---|
| `feat:` | New user-facing feature |
| `fix:` | Bug fix |
| `docs:` | Documentation only |
| `chore:` | Tooling, dependencies, configs |
| `refactor:` | Code restructuring with no behavior change |
| `test:` | Test additions or fixes |
| `perf:` | Performance improvement |
| `build:` | Build system, Makefile, Dockerfile, POM changes |

Scope suffixes are encouraged where helpful, e.g. `feat(server):`, `fix(runtime):`, `docs(agents):`.

### Code style

**Go:**
- Run `gofmt -s` and `go vet` before committing. `make test-go` runs both.
- Follow [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- One responsibility per package. The current packages (`config`, `domain`, `nats`, `store`, `dataplane`, `deploy`, `event`, `metrics`, `query`, `health`, `http`) each have a single concern — don't conflate them.

**Java:**
- Follow the existing package layout under `com.quarkloop.quark.runtime.*`.
- Quarkus best practices apply (use CDI, avoid static state, prefer constructor injection).
- Run `mvn -B clean install` before committing to ensure the build passes.

**General:**
- Comments explain **why**, not **what**. The code already says what; comments should explain the reasoning behind non-obvious decisions.
- Don't add new top-level directories without discussing in an issue first.

### Tests

- Go: every package should have `_test.go` files alongside the source. Integration tests use `// +build integration` and require a running NATS.
- Java: JUnit 5 tests live in `src/test/java/...`. Native-image compatibility tests live in `src/test/java/.../nativeimage/`.

If you add a feature, add tests. Bug-fix PRs should include a regression test.

## Reporting bugs

File issues at [github.com/quarkloop/quark/issues](https://github.com/quarkloop/quark/issues). Include:

- Component (`quark-server`, `quark-runtime`, `quark-cli`, `quark-catalog`, or `quark-nodes`)
- Version (run `quarkctl version` or check the POM version)
- Go / Java version
- NATS server version
- Minimal reproducer
- Expected vs actual behavior
- Relevant logs (Go components use zap structured logging; Java components use Quarkus logging)

## Reporting security vulnerabilities

See [SECURITY.md](./SECURITY.md). **Do not file public issues for security vulnerabilities.**

## Code of conduct

By participating in this project you agree to abide by the [Code of Conduct](./CODE_OF_CONDUCT.md).
