# Quark Declarative Format

**Status**: Architecture — TypeScript + NATS JetStream
**Date**: 2026-06-20

---

## Part I: The Specification

### Philosophy

Quark's declarative format is built on two primitives:

1. **Nodes** — The components. Defined by a name, a Docker-style URI, and configuration.
2. **NATS Subjects** — The wiring. Nodes communicate through named subjects, not direct references.

The `.quark.ts` file IS the program. Users write TypeScript. The server evaluates it via GraalJS. The result is a running system on a NATS JetStream backbone.

No arrow notation. No YAML. No custom parser. JavaScript IS the language.

---

### File Structure

Every Quark declaration file has exactly this structure:

```typescript


export default({
    name: "<system-name>",
    namespace: "<namespace>",
    nodes: {
        "<node-name>": {
            uses: "<category>/<implementation>:<version>",
            // ... configuration properties
            listens: ["<relative-subject>", ...],
            events: ["<event-type>", ...],
            onFailure: { retry: <number>, routeTo: "<node-name>" },
            timeout: "<duration>",
        },
        // ... more nodes
    },
});
```

---

### Node Definitions

A node is defined by a name and a URI. The URI follows the Docker image convention:

```
<category>/<implementation>:<version>
```

**Simple nodes** (no listeners, no events — just config):

```typescript
nodes: {
    writer: {
        uses: "store/json-writer:v1",
        path: "./out.jsonl",
        mode: "append",
        listens: ["cpu.data", "memory.data"],
    },
}
```

**Nodes with events** (publish data):

```typescript
nodes: {
    timer: {
        uses: "source/timer:v1",
        interval: "1s",
        events: ["tick"],
    },
}
```

**Nodes with failure handling**:

```typescript
nodes: {
    cpu: {
        uses: "function/cpu-profiler:v1",
        listens: ["timer.tick"],
        events: ["data"],
        onFailure: { retry: 3, routeTo: "writer" },
    },
}
```

---

### URI Versioning

Every node URI must carry a version tag, just like Docker images:

| Tag | Meaning |
|-----|---------|
| `:latest` | Most recent stable release |
| `:v1` | Pinned to major version 1 |
| `:v2.3` | Pinned to major 2, minor 3 |
| `:edge` | Bleeding-edge, potentially unstable |

---

### Communication Model

#### `listens` — What this node subscribes to

```typescript
listens: ["timer.tick", "memory.data"]
```

The engine creates a NATS JetStream Consumer for each subject. The node receives messages from these subjects via `onMessage()`.

Subjects are **relative** — the engine prefixes them with `<system>.<namespace>.`:
- `timer.tick` → `monitor.alice.timer.tick`
- `fallback.cpu` → `monitor.alice.fallback.cpu`

**Wildcards** are supported:
- `["*.data"]` — receive data from all nodes that publish a `data` event
- `["timer.>"]` — receive all events from timer

#### `events` — What this node publishes

```typescript
events: ["data", "updated"]
```

The engine creates NATS publish ACLs allowing this node to publish ONLY to:
- `monitor.alice.<nodeName>.data`
- `monitor.alice.<nodeName>.updated`

Any attempt to publish to a different subject is rejected at the NATS transport layer.

#### `onFailure` — Error handling

```typescript
onFailure: { retry: 3, routeTo: "writer" }
```

When a node fails to process a message:
1. NATS retries up to `retry` times with exponential backoff
2. After max retries, the engine publishes the error payload to `<system>.<namespace>.fallback.<nodeName>`
3. The node specified in `routeTo` must `listens` to the fallback subject to receive it

The error payload includes the original message data plus error metadata (exception message, retry count, timestamps).

---

### Complete Example: System Monitor

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
            timeout: "200ms",
            listens: ["timer.tick"],
            events: ["data"],
            onFailure: { retry: 3, routeTo: "writer" },
        },

        memory: {
            uses: "function/memory-profiler:v1",
            timeout: "200ms",
            listens: ["timer.tick"],
            events: ["data"],
            onFailure: { retry: 3, routeTo: "writer" },
        },

        writer: {
            uses: "store/json-writer:v1",
            path: "./out.jsonl",
            mode: "append",
            listens: ["cpu.data", "memory.data", "fallback.cpu", "fallback.memory"],
        },

        list: {
            uses: "store/list:v1",
            maxSize: 100,
            listens: ["cpu.data", "memory.data"],
            events: ["updated"],
        },

        stream: {
            uses: "endpoint/stream:v1",
            listens: ["list.updated"],
        },
    },
});
```

### How data flows

```
timer publishes "tick" → monitor.alice.timer.tick
    ↓ (NATS JetStream)
cpu listens ["timer.tick"] → receives tick → reads CPU → publishes "data"
    ↓ (NATS JetStream)
    ├── writer listens ["cpu.data"] → writes to file
    └── list listens ["cpu.data"] → stores → publishes "updated"
                                       ↓ (NATS JetStream)
                                       stream listens ["list.updated"] → pushes SSE

If cpu fails 3 times:
    ↓ (NATS fallback routing)
    monitor.alice.fallback.cpu
    ↓
    writer listens ["fallback.cpu"] → writes error to file
```

---

## Part II: Design Principles

| # | Principle | Rule |
|---|-----------|------|
| 1 | Everything is a Node | No hidden concepts. Every component is a node with a URI. |
| 2 | NATS is the backbone | All communication flows through NATS subjects. No direct method calls between nodes. |
| 3 | Zero coupling | No node knows about any other node. They only know their subjects. |
| 4 | Persistence by default | JetStream persists all messages. Crashes don't lose data. |
| 5 | TypeScript is the language | No YAML, no custom DSL, no arrow notation. Users write TypeScript. |
| 6 | Type safety | The `quark.d.ts` npm package provides full type definitions. |
| 7 | Multi-tenant by construction | NATS subjects encode namespace. Cross-tenant access is impossible. |
| 8 | Declarative | The file declares what nodes exist and how they communicate. The engine handles execution. |
