# Quark Platform

A universal runtime for programmable nodes, built on a **three-service architecture**: a Java/Native control plane (no GraalJS), a Go + SQLite Catalog service, and a Java/Native data plane (with GraalJS for TypeScript execution). All services communicate via an external NATS broker.

Everything in Quark — sources, functions, stores, endpoints, policies — is a **Node** identified by a Docker-style URI (`category/implementation:version`). Users declare nodes and their communication patterns in `.quark.ts` files. The control plane parses these declarations (via a comment-aware `SimpleSystemParser`, no GraalJS) and forwards them to the data plane, where GraalJS's native ESM module support evaluates TypeScript node logic over NATS.

**Multi-tenant by construction**: NATS subjects encode the namespace. Two tenants can deploy same-named systems simultaneously with zero data leakage.

---

## Architecture

### Three-Service Architecture

```
┌──────────────────┐    NATS    ┌──────────────────┐    NATS    ┌────────────────┐
│  Control Plane    │◄─────────►│  Catalog Service  │◄─────────►│  Data Plane(s)  │
│  (Java/Native)    │           │  (Go + SQLite)    │           │  (Java/Native)  │
│                   │           │                   │           │                 │
│  - REST API       │ catalog.* │  - System Store   │ quark.     │  - Node Exec    │
│  - ProcessMgr     │ subjects  │  - Node Store     │ control.*  │  - GraalJS      │
│  - Deploy Orch    │           │  - Event Store    │ quark.data.*│  - Event Fwd    │
│  - Query→NATS     │           │  - Node Registry  │           │  - Metrics Fwd  │
│  - SimpleParser   │           │  - QNP Storage    │           │                 │
└──────────────────┘           └──────────────────┘           └────────────────┘
        ▲                                                                               ▲
        │                                                                               │
        └────────────────── NATS broker (external, nats://localhost:4222) ──────────────┘
```

- **Control Plane** (`server/`): REST API, process management, deploy orchestration. Uses `SimpleSystemParser` (comment-aware, no GraalJS) to parse `.quark.ts`. Sends all persistence requests to the Catalog via NATS. Spawns data-plane processes on demand.
- **Catalog Service** (`quark-catalog/`): Standalone Go process with SQLite storage. Pure Go (`modernc.org/sqlite`, no CGO), no JNI, no GraalVM issues. Stores systems, nodes, events, source, and node packages (`.ts`/`.so` files). Performs JSONL migration on first startup.
- **Data Plane** (`runtime/`): Executes node systems. Spawned by the control plane. Includes GraalJS/Truffle for TypeScript node execution. Forwards events and metrics back via NATS.

### Native Binary Characteristics

| Binary | Size | Build time | Peak RAM | Startup | Includes GraalJS |
|--------|------|------------|----------|---------|------------------|
| Control plane (`quark-server-0.1.0-SNAPSHOT-runner`) | 76 MB | ~4 min | 3 GB | 46 ms | ❌ No |
| Data plane (`quark-runtime-runner-runner`) | 194 MB | ~9 min | 6.5 GB | 38 ms | ✅ Yes (`--macro:truffle-svm`) |
| Catalog (`quark-catalog`) | 15 MB | <5 s | <50 MB | <100 ms | n/a (Go) |

### IPC Protocol (NATS)

All control-plane ↔ data-plane communication flows through NATS:

| Direction | Subject | Purpose |
|---|---|---|
| Control → Data | `quark.control.<runtimeId>.deploy` | Deploy command (carries .quark.ts source) |
| Control → Data | `quark.control.<runtimeId>.undeploy` | Undeploy command |
| Data → Control | `quark.data.<runtimeId>.status` | Deploy/undeploy result (includes node info) |
| Data → Control | `quark.data.event.>` | Forwarded lifecycle events (NODE_CREATED, etc.) — wildcard sub |
| Data → Control | `quark.data.heartbeat.>` | Per-namespace metrics (CPU%, throughput, errors) — wildcard sub |

- `runtimeId` = `"shared"` for non-isolated namespaces, `"ns-<namespace>"` for isolated
- Serialization: JSON via Jackson
- Deploy/undeploy use NATS request-reply (synchronous, 3s timeout, 5 retries)
- Events/metrics use NATS pub/sub (asynchronous, fire-and-forget)
- Note: NATS **Core** is used (not JetStream) — no message persistence, no automatic retries, no fallback routing. The `onFailure` field is parsed but not enforced at runtime.

### Process Types

| # | Process | How spawned | Port | Purpose |
|---|---|---|---|---|
| 1 | **Control plane** | Operator (`make run-server`) | 8080 | REST API, ProcessManager, event/metrics receivers |
| 2 | **Shared data plane** | Auto-spawned by ProcessManager | 9100+ | Executes all non-isolated namespace systems |
| 3 | **Isolated data plane** | Auto-spawned when `.quark.ts` has `runtime: "isolated"` | 9101+ | Dedicated process per isolated namespace |
| 4 | **NATS server** | Operator (`nats-server`) | 4222 | Message bus for all IPC |
| 5 | **Catalog service** | Operator (`./quark-catalog/quark-catalog`) | — | Metadata store (SQLite) + node package registry |
| 6 | **Go CLI** (`quarkctl`) | Operator (`./cli/quarkctl`) | — | Talks to control plane REST API |

### Runtime Isolation

The `runtime` field in `.quark.ts` controls process isolation:

```typescript
export default {
    name: "monitor",
    namespace: "alice",
    runtime: "isolated",  // or "shared" (default)
    nodes: { ... }
};
```

- **`shared`** (default): The system runs in the shared data-plane process alongside other non-isolated namespaces.
- **`isolated`**: The system runs in a dedicated data-plane process (`runtimeId=ns-<namespace>`). The process is stopped when the namespace's last system is undeployed.

---

## Build Modes

The platform supports **two run modes**: JVM (default) and Native Image. The `RUN_MODE` env var selects which binary `make run-*` targets use.

### JVM Mode (default)

Standard `java -jar` execution. Full GraalJS support for `.quark.ts` evaluation.

```bash
make build                              # Build JVM jars + Go CLI + Catalog
make run-example                        # Run example with JVM server
make run-server                         # Start server (Ctrl+C to stop)
```

### Native Image Mode

GraalVM native executable. Starts in milliseconds, uses less memory. **Two separate native binaries** — one for the control plane (no GraalJS, 76 MB), one for the data plane (with GraalJS, 194 MB).

```bash
make build-native                       # Build BOTH native binaries + Go CLI
make build-native-server                # Control plane only (~4 min, 3 GB RAM, 76 MB)
make build-native-runtime               # Data plane with GraalJS (~9 min, 6.5 GB RAM, 194 MB)
RUN_MODE=native make run-example        # Run example with native binaries
```

**Prerequisites for native mode:**
- Oracle GraalVM 21+ with `native-image` on `$PATH`
- Set `JAVA_HOME` to the GraalVM installation
- Mandrel is sufficient for the **control plane** (no GraalJS), but the **data plane** requires Oracle GraalVM (Truffle support)

**Native mode notes:**
- **GraalJS in data plane only**: The data plane native binary includes GraalJS via `--macro:truffle-svm`. The control plane native binary excludes GraalJS entirely (uses `SimpleSystemParser`). This is what keeps the server image small. GraalJS evaluates `.ts` files via native ESM module support (`js.esm-eval-returns-exports=true`) — the platform's `.ts` files are valid ECMAScript modules with no actual TypeScript type annotations.
- **Virtual threads**: Truffle JIT compilation doesn't support virtual threads. Providers automatically use platform threads in native mode (detected via `quark.native` system property).
- **Catalog persistence**: Works in both JVM and native modes (Go + SQLite, no JNI).

### Run Mode Selection

The `RUN_MODE=jvm|native` env var (replaces the old `BUILD_MODE`) controls which binary `make run-*` targets use:

```bash
make run-example                        # JVM mode (default)
make run-example RUN_MODE=native        # Native mode
```

The `ProcessManager` automatically detects which binary is available (native or JAR) and spawns data-plane processes accordingly. Native binaries are preferred when both exist.

---

## Repository Layout

```
quark-platform/
├── AGENTS.md                            ← Guide for AI agents (READ FIRST if you're an AI)
├── README.md                            ← This file
├── Makefile                             ← All build/test/run commands (run `make help`)
├── pom.xml                              ← Parent POM (BOM management, plugin config, native profile)
├── mvnw / mvnw.cmd / .mvn/             ← Maven wrapper (DO NOT delete .mvn/wrapper/)
├── Dockerfile                           ← Clean-container build verification
├── docs/                                ← Specification documents
│
├── core/                                ← SHARED code — no GraalJS, no Quarkus
│   ├── quark-domain/                    ← Pure domain model (records, sealed interfaces)
│   ├── quark-event/                     ← Event bus + per-tenant event store SPI
│   ├── quark-registry/                  ← SPI registry for node implementations
│   ├── quark-script/                    ← SystemParser interface + SimpleSystemParser (comment-aware, no GraalJS)
│   └── quark-engine/                    ← Lifecycle, NATS wiring, DataPlaneProcess, ProcessManager
│
├── server/                              ← CONTROL PLANE — no GraalJS
│   ├── quark-app/                       ← DeployService, QueryService, NatsCatalogClient
│   ├── quark-api/                       ← JAX-RS REST endpoints + DTOs (/api/v1/...)
│   ├── quark-observability/             ← Metrics, health checks
│   └── quark-server/                    ← Quarkus runner (QuarkServer.java, native-image config)
│
├── runtime/                             ← DATA PLANE — includes GraalJS/Truffle
│   ├── quark-script/                    ← GraalJsSystemParser (GraalJS ESM-based parser)
│   ├── quark-polyglot/                  ← TypeScriptNodeFactory + PolyglotNodeRegistry (catalog pull) + JsConsole/JsConfig/JsMessage/JsPublisher bridges
│   ├── quark-app/                       ← RuntimeDeployService, DataPlaneCommandHandler
│   └── quark-runtime/                   ← Quarkus runner (QuarkRuntime.java, --macro:truffle-svm)
│   (NO providers/ subdir — runtime pulls every node from the Catalog at deploy time)
│
├── nodes/                               ← STANDARD LIBRARY (canonical node source — see nodes/README.md)
│   └── quark/                           ← 10 nodes: 5 Java (timer, cpu, memory, writer, stream) + 5 TypeScript (stdout, json-parse, map, conditional, fetch)
│       Each node dir: manifest.json + src/node.{java,ts} + build.toml + README.md
│       Build + push: quarkctl node build <uri> → quarkctl node push <uri>
│
├── quark-catalog/                       ← CATALOG service (Go + SQLite)
│   ├── cmd/quark-catalog/main.go        ← Entry point
│   └── internal/
│       ├── config/                      ← Config from flags
│       ├── natsx/                       ← NATS connection
│       ├── api/                         ← JSON request/response types
│       ├── store/                       ← SQLite persistence (pure Go)
│       └── server/                      ← NATS handlers
│
├── example/                             ← Runnable examples
│   ├── simple-streaming/                ← Multi-tenant streaming monitor
│   ├── json-pipeline/                   ← Timer → JSON parse → map → stdout
│   └── conditional-routing/             ← Conditional router with two sinks
│
└── cli/                                 ← Go-based CLI (quarkctl, with --json flag)
```

---

## Per-Namespace CPU Attribution

For shared namespaces running in the same data-plane JVM, CPU time is attributed per-namespace by measuring `ThreadMXBean.getCurrentThreadCpuTime()` inside the message handler path. The data plane forwards metrics snapshots to the control plane every 2 seconds via NATS heartbeat.

For isolated namespaces, all metrics are exact at the process level because the entire JVM serves a single namespace.

---

## How to Build & Run

### Prerequisites

- **Java 21+** (JDK — must include `javac`). For native mode: Oracle GraalVM 21+
- **Go 1.24+** (for the CLI and Catalog)
- The repo includes `mvnw` — no need to pre-install Maven
- **NATS server** on `nats://localhost:4222` (external)

### Build everything (JVM mode)

```bash
make build         # builds Java modules + Go CLI + Catalog service
```

### Build native executables

```bash
make build-native           # builds both native binaries + Go CLI + Catalog
make build-native-server    # control plane only (~4 min)
make build-native-runtime   # data plane with GraalJS (~9 min)
```

### Run the tests

```bash
make test          # runs Java + Go tests (JVM mode)
```

### Run the example

```bash
make run-example                       # JVM mode, 15-second run (default)
make run-example EXAMPLE_DURATION=30   # JVM mode, 30-second run
make run-example RUN_MODE=native       # Native mode
```

### Run the server

```bash
make run-server                       # JVM mode (port 8080)
make run-server-native                # Native mode (port 8080)
make server-dev                       # Quarkus dev mode (hot reload)
```

---

## CLI

```bash
# Deploy a .quark.ts file
quarkctl apply -f monitor.quark.ts -n alice

# List systems / nodes / namespaces
quarkctl get systems -n alice
quarkctl get nodes -n alice -s monitor
quarkctl get namespaces

# Get system / node details
quarkctl get system monitor -n alice
quarkctl get node cpu -n alice -s monitor

# Query events
quarkctl get events -n alice
quarkctl watch events -n alice

# Delete a system
quarkctl delete system monitor -n alice

# Node package registry
quarkctl node list
quarkctl node info quark/time/schedule/timer:v1
quarkctl node search timer
quarkctl node push -f my-node.ts --uri acme/transform/payments/risk-score:v1
quarkctl node pull acme/transform/payments/risk-score:v1

# Get JSON output (for AI agents)
quarkctl get system monitor -n alice --json
```

---

## REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/namespaces` | List all active namespaces |
| GET | `/api/v1/namespaces/{ns}` | Get namespace details + metrics |
| GET | `/api/v1/namespaces/{ns}/systems` | List systems in a namespace |
| GET | `/api/v1/namespaces/{ns}/systems/{name}` | Get system details |
| PUT | `/api/v1/namespaces/{ns}/systems/{name}` | Apply (declarative reconcile) |
| DELETE | `/api/v1/namespaces/{ns}/systems/{name}` | Undeploy a system |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/source` | Get the original .quark.ts source |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/nodes` | List nodes in a system |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/nodes/{node}` | Get node details |
| GET | `/api/v1/namespaces/{ns}/events` | Query events |
| GET | `/api/v1/registry` | List registered node implementations |
| GET | `/q/health/live` | Liveness check (SmallRye Health default path) |
| GET | `/q/health/ready` | Readiness check (NATS, Catalog, registry) |

---

## AI agent guide

If you're an AI agent working on this codebase, read [`AGENTS.md`](AGENTS.md) first.
