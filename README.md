# Quark Platform

A universal runtime for programmable nodes, built on Quarkus 3.x / Java 21 with an embedded NATS JetStream backbone and GraalJS-powered TypeScript evaluation.

Everything in Quark — sources, functions, stores, endpoints, policies — is a **Node** identified by a Docker-style URI (`category/implementation:version`). Users declare nodes and their communication patterns in `.quark.ts` files. The server evaluates these via GraalJS and executes them on NATS JetStream.

**Multi-tenant by construction**: NATS subjects encode the namespace. Two tenants can deploy same-named systems simultaneously with zero data leakage.

---

## Repository Layout

```
quark/
├── AGENTS.md                            ← Guide for AI agents (READ FIRST if you're an AI)
├── README.md                            ← This file
├── Makefile                             ← All build/test/run commands (run `make help`)
├── pom.xml                              ← Parent POM (BOM management, plugin config)
├── mvnw / mvnw.cmd                      ← Maven wrapper (no need to pre-install Maven)
├── Dockerfile                           ← Clean-container build verification
├── docs/                                ← Specification documents
│   ├── abstraction.md                   ← The Node concept (vision)
│   ├── DESIGN.md                        ← Design principles (v2: NATS + GraalJS)
│   ├── DECLARATION.md                   ← The .quark.ts format (syntax reference)
│   ├── node.md                      ← The Node spec (full reference)
│   ├── CLI.md                           ← CLI / server conceptual alignment
│   └── USER-STORY.md                    ← How a typical user interacts with the system
│
├── quark-core-domain/                   ← Pure domain model (records, sealed interfaces)
├── quark-core-registry/                 ← SPI registry for node implementations
├── quark-core-event/                    ← Event bus + per-tenant JSONL event store
├── quark-core-script/                   ← GraalJS layer: TS transpile, sandboxed eval, 
├── quark-core-engine/                   ← Lifecycle management (state machine, runtime registry)
├── quark-engine/                        ← Engine layer: NATS wiring (SystemRunner, publisher, subject routing)
├── quark-adapter-state/                 ← Filesystem persistence (state.json, events.jsonl, source.ts)
├── quark-app/                           ← Application services (orchestration: DeployService, QueryService, LifecycleService, HealthService)
├── quark-api/                           ← JAX-RS REST endpoints + DTOs + exception mappers
├── quark-server/                        ← Quarkus runner (Main, health checks, OpenAPI, NATS)
│
├── providers/                           ← Node implementations — SEPARATE from framework
│   ├── pom.xml                          ← Parent for all provider-* modules
│   ├── provider-stubs/                  ← Noop/memory/webhook stubs (for testing)
│   ├── provider-timer/                  ← source/timer:v1 (1-second tick source)
│   ├── provider-cpu-profiler/           ← function/cpu-profiler:v1 (CPU usage reader)
│   ├── provider-memory-profiler/        ← function/memory-profiler:v1 (heap usage reader)
│   ├── provider-list/                   ← store/list:v1 (in-memory list with cap)
│   ├── provider-json-writer/            ← store/json-writer:v1 (JSONL append to disk)
│   └── provider-streaming-endpoint/     ← endpoint/stream:v1 (HTTP SSE with shared routing)
│
├── example/                             ← Runnable examples
│   └── simple-streaming/                ← Multi-tenant streaming monitor example
│       ├── README.md                    ← How to run (TS → CLI → Server)
│       ├── system.quark.ts              ← The "program" — this is ALL the user writes
│       └── json/                        ← Output directory (server writes here)
│
└── cli/                                 ← Go-based CLI (with --json flag)
    ├── go.mod                           ← (Go 1.24+)
    ├── main.go
    ├── cmd/                             ← Cobra commands (system, node, registry, event, health)
    └── internal/                        ← HTTP client + model + output printers
```

> **Note**: An npm package for `quark.d.ts` TypeScript type definitions is planned. For now, run `quarkctl init` in your project directory to generate a local `quark.d.ts` file for IDE autocomplete.

---

## Three-Layer Architecture

```
┌─────────────────────────────────────────────────────┐
│                   GraalJS Layer                      │
│  Input:  .quark.ts file (TypeScript source)          │
│  Output: SystemConfig (plain data structure)         │
│  - Transpiles TS → JS, evaluates in sandbox          │
│  - No NATS, no providers, no execution               │
└───────────────────────┬─────────────────────────────┘
                        │ SystemConfig
                        ▼
┌─────────────────────────────────────────────────────┐
│                    Engine Layer                      │
│  Input:  SystemConfig + Provider instances           │
│  Output: Running system (NATS consumers live)        │
│  - Embedded NATS server with JetStream               │
│  - Creates consumers, ACLs, retry/fallback           │
│  - Lifecycle management, state persistence, REST API │
└───────────────────────┬─────────────────────────────┘
                        │ QuarkMessage + QuarkPublisher
                        ▼
┌─────────────────────────────────────────────────────┐
│                   Provider Layer                     │
│  Input:  QuarkMessage (incoming NATS message)        │
│  Output: QuarkPublisher.publish() (outgoing)         │
│  - Self-contained Java modules                       │
│  - Knows nothing about NATS or other nodes           │
└─────────────────────────────────────────────────────┘
```

---

## Multi-Tenancy

NATS subjects encode the namespace: `monitor.alice.*` vs `monitor.bob.*`. Isolation is enforced at:
1. **Subject namespacing** — Alice's nodes can only publish/subscribe to `monitor.alice.*`
2. **NATS ACLs** — Publish permissions restrict each node to its own subjects
3. **Engine validation** — `listens` and `events` validated to reference same-system subjects

---

## REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/registry` | List registered implementations |
| GET | `/health` | Platform health |

---

## CLI

```bash
# Deploy a .quark.ts file
quarkctl apply -f monitor.quark.ts -n alice

# List systems
quarkctl get systems -n alice

# List nodes
quarkctl get nodes -n alice -s monitor

# Watch events
quarkctl watch events -n alice -s monitor

# Get JSON output (for AI agents)
quarkctl get system monitor -n alice --json
```

---

## How to Build & Run

### Prerequisites

- **Java 21+** (JDK — must include `javac`)
- **Go 1.24+** (for the CLI)
- The repo includes `mvnw` — no need to pre-install Maven
- **NATS server** on `nats://localhost:4222` (external; the platform
  does not embed a NATS server in this release)

### Build everything

```bash
make build         # builds all Java modules + Go CLI binary
```

### Run the tests

```bash
make test          # runs Java + Go tests
```

### Run the server

```bash
make server-dev    # starts Quarkus on http://localhost:8080
```

### Run the example

```bash
make run-example                       # 15-second run (default)
make run-example EXAMPLE_DURATION=30   # 30-second run
```

---

## AI agent guide

If you're an AI agent working on this codebase, read [`AGENTS.md`](AGENTS.md) first.
