# Quark Abstractions

**Status**: Architecture — NATS JetStream + GraalJS
**Date**: 2026-06-20

---

## Layer 1: Node

Node is the base abstraction. Everything in Quark is a Node.

A Source is a Node. A Function is a Node. A Store is a Node. An Endpoint is a Node. A Policy is a Node. Anything in this system is a Node.

Every Node possesses:

### Identity
* `name` — unique identifier within the system
* `uri` — Docker-style URI: `<category>/<implementation>:<version>`
* `namespace` — tenant isolation boundary
* `system` — the system this node belongs to

### Metadata
* `labels` — key-value pairs for organization
* `annotations` — freeform descriptive information
* `createdAt` — when created
* `updatedAt` — when last modified

### State
* Current condition (created, active, paused, error, deleted)
* Current configuration (how it is set up)
* Current health (is it working)

### Behavior
* What the Node can do
* What inputs it accepts (via NATS `listens`)
* What outputs it produces (via NATS `events`)
* What side effects it has

### Communication
* `listens` — NATS subjects this node subscribes to
* `events` — NATS subjects this node publishes to
* `onFailure` — what happens when processing fails (retry + fallback routing)

### Events
* What happened
* When it happened
* Which Node was involved
* What changed

### Observability
* `health()` — liveness and readiness
* `metrics()` — performance measurements
* `recentEvents()` — event history

---

## Layer 2+: Specialized Nodes

All other Nodes inherit from Node.

They inherit Identity, Metadata, State, Behavior, Communication, Events, and Observability.

They add their own specific properties, behaviors, and events.

---

## Passive vs Active

The most important distinction among Nodes is activation.

**Passive Nodes** describe something. They exist but do not execute behavior on their own. They have State but no active Behavior.

**Active Nodes** perform behavior. They execute, consume inputs, produce outputs. They have both State and Behavior.

When a passive Node is connected to an active Node via NATS subjects, the system becomes executable.

Execution emerges from composition.

---

## Programmability

Nodes are programmable through TypeScript.

A Node may expose:
* Properties (what it is — via config in `.quark.ts`)
* Methods (what it can do — via SPI provider implementation)
* Events (what happens — via NATS `events` field)
* Policies (rules — via `onFailure` config)
* Lifecycle Rules (when to activate — via lifecycle state machine)

The platform becomes programmable because Nodes themselves are programmable.

Users do not build databases or pipelines.

Users construct programmable Node systems via TypeScript.

Those systems become executable on NATS JetStream.

Quark serves as the runtime for those systems.

---

## NATS as the Nervous System

In the current architecture, NATS JetStream is the execution model:

- **NATS consumers** handle message delivery
- **NATS subjects** ARE the routing table
- **NATS messages** ARE the data envelope
- **`listens` and `events`** replace all arrow types

NATS provides:
* **Decoupling** — nodes communicate through subjects, not direct references
* **Persistence** — JetStream streams ensure messages survive crashes
* **Resiliency** — JetStream consumers handle retries and fallback routing
* **Multi-tenancy** — subjects encode namespace, isolation is implicit

---

## User Stories as Proof

These concepts model ANY scenario:

**Date/time aggregation:**
```
Nodes: Servers, Collection, Interval
Communication: Interval publishes "tick" → Collection listens
State: Server times, Collection values
Behavior: Read time, Aggregate
Execution: Interval triggers aggregation via NATS
Events: Value collected, Collection updated
```

**System monitoring:**
```
Nodes: Log sources, Functions, Dashboard, Database
Communication: Sources publish "data" → Functions listen → Functions publish "data" → Dashboard + Database listen
State: Log data, Metrics, Dashboard state
Behavior: Parse, Aggregate, Visualize
Execution: Functions execute on schedule via NATS
Events: Logs processed, Metrics updated
```

**Document knowledge base:**
```
Nodes: Document source, Functions, API
Communication: Source publishes "data" → Functions listen → Functions publish "data" → API listens
State: Documents, Extracted text, Knowledge
Behavior: Extract, Index, Search
Execution: Ingestion runs, API serves queries
Events: Document ingested, Query answered
```

Same Node model. Different compositions. Any scenario.
