# Quark

## A Universal Runtime for Programmable Nodes

**Status**: Architecture — NATS JetStream + GraalJS
**Date**: 2026-06-20

---

## Vision

Quark is a platform for representing, composing, executing, and observing arbitrary systems through a finite set of programmable abstractions.

The goal is not to model databases, workflows, or AI agents specifically. The goal is to provide a universal node model capable of representing any information system, knowledge system, computational system, or intelligent system.

---

## Core Principles

1. **Everything is a Node** — Any entity in the system is a Node with a URI.
2. **NATS is the Backbone** — All communication flows through NATS JetStream. No direct method calls between nodes.
3. **Zero Coupling** — No node knows about any other node. They only know their NATS subjects.
4. **Persistence by Default** — JetStream persists all messages. Crashes don't lose data.
5. **TypeScript is the Language** — Users write `.quark.ts` files. No YAML, no custom DSL.
6. **Type Safety** — The `quark.d.ts` npm package provides full TypeScript type definitions.
7. **Multi-tenant by Construction** — NATS subjects encode namespace. Cross-tenant access is impossible.
8. **Declarative** — The file declares what nodes exist and how they communicate. The engine handles execution.

---

## Three-Layer Architecture

### Layer 1: GraalJS (Parsing)

- **Input**: `.quark.ts` file (TypeScript source)
- **Output**: `SystemConfig` (plain data structure)
- **Responsibility**: Transpile TS → JS, evaluate in sandboxed GraalJS Context, extract config
- **Does NOT**: Touch NATS, instantiate providers, execute anything

### Layer 2: Engine (Execution)

- **Input**: `SystemConfig` + Provider instances
- **Output**: Running system (NATS consumers/publishers live)
- **Responsibility**: Embedded NATS server, JetStream streams/consumers/ACLs, lifecycle management, retry/fallback, state persistence, REST API
- **Does NOT**: Parse TypeScript, implement node logic

### Layer 3: Providers (Implementation)

- **Input**: `QuarkMessage` (incoming NATS message)
- **Output**: `QuarkPublisher.publish()` (outgoing NATS message)
- **Responsibility**: Implement node behavior (timer ticks, CPU reads, file writes, HTTP serving)
- **Does NOT**: Know about NATS subjects, other nodes, or the engine internals

---

## Node Model

### Node Categories

| Category | URI prefix | Role | SPI Interface |
|----------|-----------|------|---------------|
| Source | `source/` | Produces data autonomously | `SourceProvider` |
| Function | `function/` | Transforms data on receipt | `FunctionProvider` |
| Store | `store/` | Persists data | `StoreProvider` |
| Endpoint | `endpoint/` | External interface (HTTP, SSE) | `EndpointProvider` |
| Policy | `policy/` | Governance rules | `PolicyProvider` |

### Communication Model

Nodes communicate exclusively through NATS subjects:

- **`listens`**: Array of subjects the node subscribes to (NATS Consumer)
- **`events`**: Array of event types the node publishes (NATS Publisher ACL)
- **`onFailure`**: Retry count + fallback routing target

No node has a direct reference to any other node. The NATS subject is the only coupling — and it's a string, not a code-level dependency.

### Subject Naming

```
<system>.<namespace>.<node>.<event>
```

Example: `monitor.alice.timer.tick`

This encodes:
- Which system (`monitor`)
- Which namespace/tenant (`alice`)
- Which node (`timer`)
- What event (`tick`)

Multi-tenancy is implicit — Alice's subjects can never overlap with Bob's.

---

## Passive vs Active

| Passive Nodes | Active Nodes |
|---------------|-------------|
| Source, Store, Endpoint, Policy | Function |
| Exist, hold state, expose interfaces | Execute behavior: receive, transform, produce |
| State only | State + behavior |

Execution emerges from composition — a Source publishing events that a Function listens to creates an executable pipeline through NATS.

---

## Design Principles Summary

| Principle | Rule |
|-----------|------|
| Everything is a Node | No hidden concepts. Every component has a URI. |
| NATS is the backbone | All communication via subjects. No direct calls. |
| Zero coupling | Nodes know subjects, not other nodes. |
| Persistence by default | JetStream persists everything. |
| TypeScript is the language | No YAML, no DSL, no arrows. |
| Type safety | npm package provides full type definitions. |
| Multi-tenant by construction | Subjects encode namespace. |
| Declarative | Declare what, not how. Engine handles execution. |
