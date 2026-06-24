# Quark Platform

A universal runtime for programmable nodes, built on Quarkus 3.x / Java 21 with NATS JetStream backbone, GraalJS-powered TypeScript evaluation, and a standalone Catalog service for persistence.

Everything in Quark — sources, functions, stores, endpoints, policies — is a **Node** identified by a Docker-style URI (`category/implementation:version`). Users declare nodes and their communication patterns in `.quark.ts` files. The server evaluates these via GraalJS and executes them on NATS JetStream.

**Multi-tenant by construction**: NATS subjects encode the namespace. Two tenants can deploy same-named systems simultaneously with zero data leakage.

---

## Architecture

### Three-Service Architecture

The platform consists of three services communicating via NATS:

```
┌──────────────────┐    NATS    ┌──────────────────┐    NATS    ┌────────────────┐
│  Control Plane    │◄─────────►│  Catalog Service  │◄─────────►│  Data Plane(s)  │
│  (Java/Native)    │           │  (Go + SQLite)    │           │  (Java/Native)  │
│                   │           │                   │           │                 │
│  - REST API       │ catalog.* │  - System Store   │ registry.*│  - Node Exec    │
│  - ProcessMgr     │ subjects  │  - Node Store     │  subjects │  - Event Fwd    │
│  - Deploy Orch    │           │  - Event Store    │           │  - Metrics Fwd  │
│  - Query→NATS     │           │  - Node Registry  │           │                 │
│  - Node Cache     │           │  - QNP Storage    │           │                 │
└──────────────────┘           └──────────────────┘           └────────────────┘
```

- **Control Plane** (Java/Native): REST API, process management, deploy orchestration. Sends all persistence requests to the Catalog via NATS.
- **Catalog Service** (Go + SQLite): Standalone metadata store and node package registry. Pure Go, no JNI, no GraalVM issues. Stores systems, nodes, events, source, and node packages (`.so`/`.ts` files).
- **Data Plane** (Java/Native): Executes node systems. Spawned by the control plane. Forwards events and metrics back via NATS.

### IPC Protocol (NATS)

All control-plane ↔ data-plane communication flows through NATS:

| Direction | Subject | Purpose |
|---|---|---|
| Control → Data | `quark.control.<runtimeId>.deploy` | Deploy command (carries .quark.ts source) |
| Control → Data | `quark.control.<runtimeId>.undeploy` | Undeploy command |
| Data → Control | `quark.data.status.<runtimeId>` | Deploy/undeploy result (includes node info) |
| Data → Control | `quark.data.event.<runtimeId>` | Forwarded lifecycle events (NODE_CREATED, etc.) |
| Data → Control | `quark.data.heartbeat.<runtimeId>` | Per-namespace metrics (CPU%, throughput, errors) |

- `runtimeId` = `"shared"` for non-isolated namespaces, `"ns-<namespace>"` for isolated
- Serialization: JSON via Jackson
- Deploy/undeploy use NATS request-reply (synchronous, 3s timeout, 5 retries)
- Events/metrics use NATS pub/sub (asynchronous, fire-and-forget)

### Process Types

| # | Process | How spawned | Port | Purpose |
|---|---|---|---|---|
| 1 | **Control plane** | Operator (`make run-server` or `make run-example`) | 8080 | REST API, ProcessManager, event/metrics receivers |
| 2 | **Shared data plane** | Auto-spawned by ProcessManager | 9100+ | Executes all non-isolated namespace systems |
| 3 | **Isolated data plane** | Auto-spawned when `.quark.ts` has `runtime: "isolated"` | 9101+ | Dedicated JVM per isolated namespace |
| 4 | **NATS server** | Operator (`nats-server`) | 4222 | Message bus for all IPC |
| 5 | **Catalog service** | Operator (`make build-catalog` + run) | — | Metadata store (SQLite) + node package registry |
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

The platform supports **two build modes**: JVM (default) and Native Image.

### JVM Mode (default)

Standard `java -jar` execution. Full GraalJS support for `.quark.ts` evaluation.

```bash
make build                              # Build JVM jar + Go CLI
make run-example                        # Run example with JVM server
make run-server                         # Start server (Ctrl+C to stop)
```

### Native Image Mode

GraalVM/Mandrel native executable. Starts in milliseconds, uses less memory.

```bash
make build-native                       # Build native executable + Go CLI
BUILD_MODE=native make run-example      # Run example with native server
```

**Prerequisites for native mode:**
- Oracle GraalVM 21+ with `native-image` on `$PATH` (Mandrel is NOT sufficient — Truffle support requires Oracle GraalVM)
- Set `JAVA_HOME` to the GraalVM installation

**Native mode limitations:**
- **GraalJS**: Fully supported via `--macro:truffle-svm`. The GraalJS/Truffle interpreter is statically compiled into the native binary, enabling full `.quark.ts` evaluation at runtime. Requires Oracle GraalVM (not Mandrel) for the native build.
- **Virtual threads**: Truffle JIT compilation doesn't support virtual threads. Providers automatically use platform threads in native mode (detected via `quark.native` system property).
- **Catalog persistence works in both JVM and native modes (Go + SQLite, no JNI).

### Build Mode Selection

All Makefile targets accept `BUILD_MODE=jvm|native`:

```bash
make build BUILD_MODE=native            # Build in native mode
make run-example BUILD_MODE=native      # Run example in native mode
make test BUILD_MODE=native             # Test in native mode (JVM tests only)
```

The `ProcessManager` automatically detects which binary is available (native or JAR) and spawns data-plane processes accordingly. Native binaries are preferred when both exist.

---

## Repository Layout

```
quark/
├── AGENTS.md                            ← Guide for AI agents (READ FIRST if you're an AI)
├── README.md                            ← This file
├── Makefile                             ← All build/test/run commands (run `make help`)
├── pom.xml                              ← Parent POM (BOM management, plugin config, native profile)
├── mvnw / mvnw.cmd                      ← Maven wrapper (no need to pre-install Maven)
├── Dockerfile                           ← Clean-container build verification
├── docs/                                ← Specification documents
│
├── quark-core-domain/                   ← Pure domain model (records, sealed interfaces)
├── quark-core-registry/                 ← SPI registry for node implementations
├── quark-core-event/                    ← Event bus + per-tenant JSONL event store
├── quark-core-script/                   ← GraalJS + SimpleSystemParser (TS transpile, sandboxed eval)
├── quark-core-engine/                   ← Lifecycle management, metrics, data-plane IPC protocol
├── quark-engine/                        ← Engine layer: NATS wiring (publisher, subject routing)
├── quark-catalog/                        ← Catalog service (Go + SQLite) — metadata store + node registry
├── quark-app/                           ← Application services (DeployService, QueryService, ProcessManager)
├── quark-api/                           ← JAX-RS REST endpoints + DTOs + exception mappers
├── quark-server/                        ← Quarkus runner (Main, health checks, native-image config)
│
├── providers/                           ← Node implementations — SEPARATE from framework
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
│
└── cli/                                 ← Go-based CLI (with --json flag)
```

---

## Per-Namespace CPU Attribution

For shared namespaces running in the same data-plane JVM, CPU time is attributed per-namespace by measuring `ThreadMXBean.getCurrentThreadCpuTime()` inside the message handler path. The data plane forwards metrics snapshots to the control plane every 2 seconds via NATS heartbeat.

```
CLI: quarkctl get stats
NAMESPACE     SYS NODES    CPU%     MSG/S     ERR/S   HEAP   NONH       OK       BAD
alice           1     6    0.6%      8.0     0.00    26MB     64MB        6        0
```

For isolated namespaces, all metrics are exact at the process level because the entire JVM serves a single namespace.

---

## How to Build & Run

### Prerequisites

- **Java 21+** (JDK — must include `javac`). For native mode: Mandrel 23.1+ or GraalVM 21+
- **Go 1.24+** (for the CLI)
- The repo includes `mvnw` — no need to pre-install Maven
- **NATS server** on `nats://localhost:4222` (external)
- The Catalog service is built from source (`make build-catalog`)

### Build everything (JVM mode)

```bash
make build         # builds Java modules + Go CLI + Catalog service
```

### Build native executable

```bash
make build-native  # builds native executable + Go CLI binary
```

### Run the tests

```bash
make test          # runs Java + Go tests (JVM mode)
```

### Run the example

```bash
make run-example                       # JVM mode, 15-second run (default)
make run-example EXAMPLE_DURATION=30   # JVM mode, 30-second run
BUILD_MODE=native make run-example     # Native mode
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

# List systems
quarkctl get systems -n alice

# List nodes
quarkctl get nodes -n alice -s monitor

# Watch events
quarkctl watch events -n alice -s monitor

# Get real-time stats (CPU%, throughput, errors per namespace)
quarkctl get stats
quarkctl get stats --watch

# Get JSON output (for AI agents)
quarkctl get system monitor -n alice --json
```

---

## REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/namespaces` | List all active namespaces |
| GET | `/api/v1/namespaces/{ns}` | Get namespace details + metrics |
| POST | `/api/v1/namespaces/{ns}/systems` | Deploy a system |
| PUT | `/api/v1/namespaces/{ns}/systems/{name}` | Apply (declarative reconcile) |
| DELETE | `/api/v1/namespaces/{ns}/systems/{name}` | Undeploy a system |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/source` | Get the original .quark.ts source |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/nodes` | List nodes in a system |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/nodes/{node}` | Get node details |
| GET | `/api/v1/namespaces/{ns}/events` | Query events |
| GET | `/registry` | List registered node implementations |
| GET | `/health/live` | Liveness check |
| GET | `/health/ready` | Readiness check (NATS, Catalog, registry) |

---

## AI agent guide

If you're an AI agent working on this codebase, read [`AGENTS.md`](AGENTS.md) first.
