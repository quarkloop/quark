# Quark

A universal runtime for programmable nodes, built on a three-service architecture: a Go control plane, a Go + SQLite Catalog service, and a Java/Native data plane with GraalJS for TypeScript execution. All services communicate via an external NATS broker.

## Overview

Everything in Quark — timers, profilers, parsers, writers, endpoints, policies — is a **Node** identified by a Docker-style URI (`<namespace>/<domain>/<subdomain>/<node>:<version>`). Users declare nodes and their communication patterns in `.quark.ts` files. The control plane persists these declarations verbatim and forwards them to the data plane, where GraalJS's native ESM module support evaluates TypeScript node logic.

**Multi-tenant by construction**: NATS subjects encode the namespace. Two tenants can deploy same-named systems simultaneously with zero data leakage.

## Features

- **Three-service architecture** — Go control plane (Fiber + nats.go), Go Catalog (SQLite, pure Go no CGO), Java data plane (Quarkus + GraalJS/Truffle)
- **Multi-tenant by construction** — NATS subjects encode the namespace; zero data leakage between tenants
- **TypeScript-native node execution** — GraalJS evaluates `.quark.ts` files via native ESM module support; no separate transpile step
- **Declarative system definitions** — users write `.quark.ts` files, the platform handles deploy/undeploy lifecycle
- **Node registry / docker-image model** — nodes are built, pushed to the Catalog, and pulled on demand by the runtime; no runtime rebuild to add a node
- **Runtime isolation modes** — `shared` (default, multi-tenant in one process) or `isolated` (dedicated process per namespace)
- **Native image support** — GraalVM native executables for both control plane (76 MB, no GraalJS) and data plane (194 MB, with GraalJS via `--macro:truffle-svm`)
- **REST API + CLI** — `quarkctl` for operators, REST for programmatic integrations
- **Per-namespace CPU attribution** — `ThreadMXBean.getCurrentThreadCpuTime()` measured per message handler for shared namespaces

## Installation

This is a multi-language platform — there is no single install command. Clone and build:

```bash
git clone https://github.com/quarkloop/quark.git
cd quark
make build         # builds Java modules + Go CLI + Catalog service
```

See [BUILD.md](./BUILD.md) for full prerequisites (JDK 21+, Go 1.24+, NATS server, optional GraalVM for native mode) and per-component build instructions.

## Quick start

```bash
# 1. Start NATS (external):
nats-server &

# 2. Build everything (JVM mode):
make build

# 3. Run the multi-tenant streaming example (15 seconds):
make run-example
```

This deploys `example/simple-streaming/system.quark.ts` under namespace `alice`, observes the streaming output for 15 seconds, then undeploys and shuts down cleanly.

For native mode: `make build-native && make run-example RUN_MODE=native`.

## Documentation

- [Architecture](./ARCHITECTURE.md) — three-service model, runtime isolation, process types, node lifecycle
- [API reference](./API.md) — REST endpoints and CLI commands
- [Wire protocol](./PROTOCOL.md) — NATS subjects for control-plane ↔ data-plane communication
- [Build & development](./BUILD.md) — prerequisites, JVM vs native mode, Makefile targets, Docker verification
- [Changelog](./CHANGELOG.md) — release history
- [Contributing](./CONTRIBUTING.md) — development setup, PR workflow, code style per language
- [AI agent guide](./AGENTS.md) — read this first if you're an AI working on the codebase

The documentation website (Next.js site in [`docs/`](./docs/)) hosts the human-facing tutorials and conceptual deep-dives.

## Compatibility

| Component | Language | Version |
|---|---|---|
| Control plane (`quark-server/`) | Go | 1.24+ |
| Catalog service (`quark-catalog/`) | Go | 1.24+ |
| CLI (`quark-cli/`) | Go | 1.24+ |
| Data plane (`quark-runtime/`) | Java | JDK 21+ (GraalVM 21+ for native mode) |
| NATS server | — | 2.10+ |
| Build system | — | Maven 3.9+ (wrapper included) |

## Contributing

Pull requests are welcome. See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup, commit message conventions, and per-language code style rules. By participating you agree to abide by the [Code of Conduct](./CODE_OF_CONDUCT.md).

## License

This project is licensed under the Apache License, Version 2.0 — see the [LICENSE](./LICENSE) file for details.
