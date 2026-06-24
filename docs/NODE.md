# Quark Node Specification

**Status**: Architecture — Three-Service (Control Plane / Catalog / Data Plane) + NATS Core + GraalJS
**Date**: 2026-06-24

---

## 1. Introduction

Quark is a universal runtime for programmable nodes. Everything in Quark — sources, functions, stores, endpoints, policies — is a **Node** identified by a Docker-style URI (`category/implementation:version`).

Users write **TypeScript files** (`.quark.ts`) that declare nodes and their communication patterns. The control plane parses these files via **`SimpleSystemParser`** (regex-based, no GraalJS) and forwards them to a data-plane process, where **GraalJS** evaluates TypeScript node logic over an **external NATS Core** backbone. Nodes communicate exclusively through NATS subjects — no node knows about any other node directly.

See `docs/DESIGN.md` for the full three-service architecture (control plane, Catalog, data plane).

---

## 2. Three-Service Architecture

Quark runs as three cooperating services (see `docs/DESIGN.md` for the full diagram):

| Service | Tier | Includes GraalJS? | Responsibility |
|---------|------|-------------------|----------------|
| **Control plane** (`server/`) | `server/` | ❌ No | REST API, deploy orchestration, parses `.quark.ts` with `SimpleSystemParser` (regex), spawns data-plane processes |
| **Catalog** (`quark-catalog/`) | (Go) | n/a | SQLite-backed metadata store (systems, nodes, events, source, registry) |
| **Data plane** (`runtime/`) | `runtime/` | ✅ Yes | Executes nodes via GraalJS + providers; connects to external NATS server |

### Parse-time vs Execute-time

The `.quark.ts` file is parsed **twice**:

1. **Parse-time (control plane)**: `SimpleSystemParser` (in `core/quark-script`) extracts system name, namespace, runtime mode, and node definitions using regex. No GraalJS, no JS evaluation. This produces a `SystemDefinition` record used for metadata persistence and deploy routing.

2. **Execute-time (data plane)**: `GraalJsSystemParser` (in `runtime/quark-script`) evaluates the TypeScript via GraalJS to extract runtime config (e.g. dynamic config values that require JS evaluation). `TypeScriptNodeFactory` (in `runtime/quark-polyglot`) uses GraalJS to execute TypeScript node implementations.

### Provider Layer

Providers are self-contained Java modules in `runtime/providers/provider-*`:
- Input: `QuarkMessage` (incoming NATS message)
- Output: `QuarkPublisher.publish()` (outgoing NATS message)
- Implements one SPI interface (`SourceProvider`, `FunctionProvider`, `StoreProvider`, `EndpointProvider`, `PolicyProvider`)
- Knows nothing about NATS subjects, other nodes, or the engine internals
- No static/shared state (instance-per-node)

---

## 3. The `.quark.ts` File

### 3.1 Structure

```typescript


export default({
    name: "monitor",
    namespace: "alice",

    nodes: {
        timer: {
            uses: "source/timer:v1",
            interval: "1s",
            events: ["tick"],
        },
        cpu: {
            uses: "function/cpu-profiler:v1",
            listens: ["timer.tick"],
            events: ["data"],
            onFailure: { retry: 3, routeTo: "writer" },
        },
        // ...
    },
});
```

### 3.2 Fields

#### System-level fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | System name (e.g., `"monitor"`). Must be lowercase alphanumeric + hyphens. |
| `namespace` | `string` | Yes | Tenant namespace (e.g., `"alice"`). Enforces isolation. |
| `nodes` | `Record<string, NodeDefinition>` | Yes | Map of node name → definition. |

#### NodeDefinition fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `uses` | `string` | Yes | Node URI: `<category>/<implementation>:<version>` |
| `[config]` | `unknown` | No | Any configuration properties for the node (e.g., `interval: "1s"`, `path: "./out.jsonl"`) |
| `listens` | `string[]` | No | NATS subjects this node subscribes to (relative, e.g., `["timer.tick"]`). Resolved to `<system>.<namespace>.timer.tick` by the engine. |
| `events` | `string[]` | No | Event types this node publishes (e.g., `["data", "updated"]`). Engine creates ACLs allowing publish to `<system>.<namespace>.<nodeName>.<eventType>`. |
| `onFailure` | `OnFailureConfig` | No | Retry and fallback routing configuration. |
| `timeout` | `string` | No | Processing timeout (e.g., `"200ms"`). If the provider doesn't respond in time, the message is NAK'd and retried. |

#### OnFailureConfig

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `retry` | `number` | Yes | Maximum retry attempts before routing to fallback. |
| `routeTo` | `string` | Yes | Node name to receive failed messages. The engine publishes the error payload to `<system>.<namespace>.fallback.<nodeName>`, and the target node must `listens` to it. |

### 3.3 Subject Naming Convention

All NATS subjects follow the format:

```
<system>.<namespace>.<node>.<event>
```

Examples:
```
monitor.alice.timer.tick          → timer's tick event
monitor.alice.cpu.data            → cpu's data output
monitor.alice.memory.data         → memory's data output
monitor.alice.list.updated        → list's updated event
monitor.alice.fallback.cpu        → failed messages from cpu
monitor.alice.fallback.memory     → failed messages from memory
```

The `listens` field in a node definition uses **relative** subjects. The engine resolves them to full subjects by prefixing with `<system>.<namespace>.`:

- `listens: ["timer.tick"]` → subscribes to `monitor.alice.timer.tick`
- `listens: ["cpu.data", "fallback.cpu"]` → subscribes to both `monitor.alice.cpu.data` and `monitor.alice.fallback.cpu`
- `listens: ["*.data"]` → wildcard subscription to all `*.data` events in the system

### 3.4 Wildcard Subscriptions

NATS supports wildcard subjects:
- `*` matches a single token: `monitor.alice.*.data` matches `cpu.data`, `memory.data`
- `>` matches multiple tokens: `monitor.alice.>` matches everything in the system

Users can use wildcards in `listens`:
```typescript
listens: ["*.data"]  // receive data from all nodes that publish "data"
```

---

## 4. Node Categories

| Category | URI prefix | Role | SPI Interface |
|----------|-----------|------|---------------|
| Source | `source/` | Produces data autonomously (timers, file watchers, webhooks) | `SourceProvider` |
| Function | `function/` | Transforms data on receipt | `FunctionProvider` |
| Store | `store/` | Persists data, serves queries | `StoreProvider` |
| Endpoint | `endpoint/` | External interface (HTTP, SSE, gRPC) | `EndpointProvider` |
| Policy | `policy/` | Governance rules | `PolicyProvider` |

### 4.1 Passive vs Active

| Type | Categories | Behavior |
|------|-----------|----------|
| Passive | Source, Store, Endpoint, Policy | Exist, hold state, expose interfaces. Do not execute behavior on their own. |
| Active | Function | Execute behavior: receive input, transform, produce output. |

Execution emerges from composition — a Source publishing events that a Function listens to creates an executable pipeline through NATS.

---

## 5. SPI Interfaces

### 5.1 QuarkMessage

```java
public interface QuarkMessage {
    String subject();                    // Full NATS subject
    Map<String, Object> payload();       // Message data
    Map<String, String> headers();       // Metadata
    Instant timestamp();                 // When NATS received it
    String systemName();                 // "monitor"
    String namespace();                  // "alice"
    String nodeName();                   // "cpu"
}
```

### 5.2 QuarkPublisher

```java
public interface QuarkPublisher {
    // Publish an event. Engine resolves to full subject:
    // "<system>.<namespace>.<nodeName>.<event>"
    // ACL enforced: can only publish events declared in "events: [...]"
    void publish(String event, Map<String, Object> payload);
}
```

### 5.3 SourceProvider

```java
public interface SourceProvider {
    void start(QuarkPublisher publisher, NodeConfig config);
    void stop();
}
```

Sources are autonomous — they produce data on their own schedule and publish via the `QuarkPublisher`.

### 5.4 FunctionProvider

```java
public interface FunctionProvider {
    void onMessage(QuarkMessage message, QuarkPublisher publisher);
}
```

Functions are reactive — they receive messages via `onMessage`, process them, and publish results via `publisher.publish()`.

### 5.5 StoreProvider

```java
public interface StoreProvider {
    void onMessage(QuarkMessage message, QuarkPublisher publisher);
}
```

Stores are reactive — same interface as functions. They persist data and optionally publish events (e.g., `updated`).

### 5.6 EndpointProvider

```java
public interface EndpointProvider {
    void start(QuarkPublisher publisher, NodeConfig config);
    void onMessage(QuarkMessage message, QuarkPublisher publisher);
    void stop();
}
```

Endpoints are hybrid — they start their own server (HTTP, SSE) AND receive messages from NATS. An SSE endpoint listens for `list.updated` and pushes to connected HTTP clients.

### 5.7 PolicyProvider

```java
public interface PolicyProvider {
    void onMessage(QuarkMessage message, QuarkPublisher publisher);
}
```

Policies intercept messages — they receive, evaluate, and either allow (publish to original target) or deny (drop or route to fallback).

---

## 6. NATS Messaging Design

### 6.1 External NATS Server

The Quark server connects to an **external** NATS server at `nats://localhost:4222` (configurable via `quark.nats.url`). Start one before deploying systems:

```bash
# Using Docker:
docker run -p 4222:4222 nats:latest

# Using Homebrew (macOS):
brew install nats-server && nats-server
```

If no NATS server is available, the platform operates in **degraded mode**: systems can be deployed and tracked in the runtime registry, lifecycle events are emitted, but message flow between nodes is disabled. Start a NATS server and redeploy to enable full functionality.

### 6.2 Core NATS Messaging (v0.1)

This release uses **NATS Core** messaging (not JetStream). The implications:

- **No message persistence** — messages are delivered to currently-connected subscribers only. If a subscriber is offline, the message is lost.
- **No automatic retries** — `onFailure.retry` is parsed but not yet enforced. A failed `onMessage()` call logs the error and acknowledges the message.
- **No fallback routing** — `onFailure.routeTo` is parsed but the `<sys>.<ns>.fallback.<node>` subject is never published to.
- **ACLs are enforced in-process** — `NatsQuarkPublisher` checks that the event is in the node's declared `events` list before publishing. This is a defense-in-depth measure, not a NATS transport-level ACL.

### 6.3 Subscriptions

For each node with `listens`, the engine creates a NATS core subscription via `Connection.createDispatcher()` + `dispatcher.subscribe(fullSubject)`. The dispatcher's message handler:

1. Wraps the NATS message in a `NatsQuarkMessage`
2. Dispatches to the provider's `onMessage()`
3. On success: `msg.ack()` (no-op in core NATS)
4. On exception: logs the error and `msg.nak()` (no-op in core NATS)

### 6.4 Planned: JetStream Upgrade

A future release will upgrade to JetStream for:
- Persistent streams per system (`<sys>.<ns>.>`)
- Durable consumers with `MaxDeliver` and exponential backoff (wired to `onFailure.retry`)
- Fallback subject routing after max retries (wired to `onFailure.routeTo`)
- Transport-level publish ACLs

The `onFailure` config field is parsed and stored today so that `.quark.ts` files are forward-compatible with the JetStream upgrade.

---

## 7. Lifecycle State Machine

Every node follows:

```
CREATING → ACTIVE → PAUSED → DRAINING → ARCHIVED → DELETED
                ↘ ERROR → RECOVERING ↗
```

State transitions emit `NODE_STATE_CHANGED` events. The lifecycle is managed by the engine — providers don't control it.

When a node is paused:
- Its lifecycle state transitions to `PAUSED`
- The NATS subscription remains active (messages continue to be delivered and acknowledged)
- A future JetStream upgrade will pause the consumer so messages accumulate for later delivery

---

## 8. Multi-Tenancy

### 8.1 Namespace Isolation

NATS subjects encode the namespace: `monitor.alice.*` vs `monitor.bob.*`. Isolation is enforced at three levels:

1. **Subject namespacing**: Alice's nodes can only publish/subscribe to `monitor.alice.*` subjects
2. **NATS ACLs**: Publish permissions restrict each node to its own subjects
3. **Engine validation**: The `listens` and `events` fields are validated to only reference subjects within the same system+namespace

### 8.2 Multiple Systems Per Namespace

A namespace can contain multiple independent systems:

```
Namespace: alice
  ├── System: monitor        (subjects: monitor.alice.*)
  ├── System: log-processor  (subjects: log-processor.alice.*)
  └── System: alerting       (subjects: alerting.alice.*)
```

Systems in the same namespace share the namespace for management purposes (list, health, events queries) but are isolated at the NATS subject level.

---

## 9. Persistence Layout

As of v8, all persistence is delegated to the **Catalog service** (Go + SQLite). The Catalog database lives at:

```
$QUARK_STATE_ROOT/
├── catalog.db                                  SQLite database (Catalog service)
│                                               Tables: systems, nodes, events,
│                                                       node_packages, registry,
│                                                       catalog_meta
└── dataplane-logs/                             Per-runtimeId data-plane stdout/stderr
    └── dataplane-<runtimeId>.log
```

The legacy on-disk layout (`$QUARK_STATE_ROOT/systems/<ns>/<sys>/source.ts` + `events.jsonl`) is **migrated automatically** on first Catalog startup via `store.MigrateLegacy()` — the `systems/` directory is renamed to `systems.backup/` after migration.

> **Note**: `system.meta.json` and `state.json` mentioned in earlier specs are no longer planned — all metadata lives in the Catalog's SQLite tables.

---

## 10. REST API

All endpoints are scoped under `/api/v1/` on the control plane (port 8080):

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

## 11. CLI

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
quarkctl node info source/timer:v1
quarkctl node search timer
quarkctl node push -f my-node.ts --uri source/my-node:v1
quarkctl node pull source/my-node:v1

# Get JSON output (for AI agents)
quarkctl get system monitor -n alice --json
```
