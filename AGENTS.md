# AGENTS.md — Guide for AI Agents Working on Quark

> **If you read nothing else, read this:** This is a Java 21 + Quarkus 3.x server with a Go CLI. **The `.quark.ts` file IS the program.** Users write only TypeScript — they never touch Java code. The CLI sends the TypeScript to the server, which evaluates it via GraalJS, then executes it on an embedded NATS JetStream backbone. The three layers (GraalJS, Engine, Providers) are STRICTLY SEPARATED. Read the [Architecture](#architecture) section before making any cross-module change.

---

## Quick orientation

```
quark/
├── AGENTS.md                         ← this file — READ FIRST
├── README.md                         ← human-facing project overview
├── Makefile                          ← all build/test/run commands
├── pom.xml                           ← Maven parent POM
├── mvnw, mvnw.cmd, .mvn/            ← Maven wrapper
│
├── docs/                             ← specifications (read these to understand intent)
│   ├── abstraction.md                ← the Node concept (vision)
│   ├── DESIGN.md                     ← design principles (v2: NATS + GraalJS)
│   ├── DECLARATION.md                ← the .quark.ts format (syntax reference)
│   ├── node.md                   ← the Node spec (full reference)
│   ├── CLI.md                        ← CLI / server conceptual alignment
│   └── USER-STORY.md                 ← how a typical user interacts with the system
│
├── quark-core-domain/                ← Pure domain model (records, sealed interfaces)
├── quark-core-registry/              ← SPI registry for node implementations
├── quark-core-event/                 ← Event bus + per-tenant JSONL event store
├── quark-core-script/                ← GraalJS layer: TS transpile, sandboxed eval, 
├── quark-core-engine/                ← Lifecycle management (state machine, runtime registry)
├── quark-engine/                     ← Engine layer: NATS wiring (SystemRunner, NatsQuarkPublisher, subject routing)
├── quark-adapter-state/              ← Filesystem persistence (state.json, events.jsonl, source.ts)
├── quark-app/                        ← Application services (DeployService, QueryService, LifecycleService, HealthService)
├── quark-api/                        ← JAX-RS REST endpoints + DTOs + exception mappers
├── quark-server/                     ← Quarkus runner (Main, health checks, OpenAPI, NATS)
│
├── providers/                        ← Node implementations — SEPARATE from framework
│   ├── pom.xml                       ← Parent for all provider-* modules
│   ├── provider-stubs/               ← Noop/memory/webhook stubs (for testing)
│   ├── provider-timer/               ← source/timer:v1
│   ├── provider-cpu-profiler/        ← function/cpu-profiler:v1
│   ├── provider-memory-profiler/     ← function/memory-profiler:v1
│   ├── provider-list/                ← store/list:v1
│   ├── provider-json-writer/         ← store/json-writer:v1
│   └── provider-streaming-endpoint/  ← endpoint/stream:v1
│
├── example/                          ← Runnable examples
│   └── simple-streaming/             ← Multi-tenant streaming monitor example
│       ├── README.md
│       ├── system.quark.ts           ← The "program" — this is ALL the user writes
│       └── json/                     ← Output directory (server writes here)
│
└── cli/                              ← Go-based CLI (quarkctl)
    ├── go.mod                        ← (Go 1.24+)
    ├── main.go
    ├── cmd/                          ← Cobra commands (system, node, registry, event, health)
    └── internal/                     ← HTTP client + model + output printers
```

---

## Architecture

### The fundamental separation

Three layers with strict boundaries:

| Layer | Module(s) | Depends on | Does NOT depend on |
|-------|-----------|------------|-------------------|
| **GraalJS** | `quark-core-script` | GraalJS SDK, esbuild | NATS, providers, engine |
| **Engine** | `quark-engine`, `quark-core-engine` | NATS client, domain model | GraalJS, providers |
| **Provider** | `providers/provider-*` | New SPI interfaces, domain model | NATS, GraalJS, engine, other providers |

**Never violate these boundaries:**
- GraalJS layer must never import NATS or provider classes
- Engine layer must never import GraalJS classes
- Provider layer must never import NATS or engine classes

### NATS as the backbone

All node communication flows through NATS JetStream. There is no direct method calling between nodes.

- **`listens`** → NATS JetStream Consumer (durable, persistent)
- **`events`** → NATS publish ACL (restricts to specific subjects)
- **`onFailure`** → NATS consumer retry policy + fallback subject routing
- **Subjects** → `<system>.<namespace>.<node>.<event>` (encodes multi-tenancy)

### Multi-tenancy enforcement

NATS subjects encode the namespace: `monitor.alice.*` vs `monitor.bob.*`. Isolation is enforced at:
1. **Subject namespacing** — Alice's nodes can only publish/subscribe to `monitor.alice.*`
2. **NATS ACLs** — Publish permissions restrict each node to its own subjects
3. **Engine validation** — `listens` and `events` validated to reference only same-system subjects

---

## Common pitfalls

0. **Never create standalone runners or require users to write Java code.** The `.quark.ts` file IS the program. Users deploy via CLI (`quarkctl system deploy -f file.quark.ts`). The server is the interpreter.

1. **Never add cross-namespace methods.** All lookups require a `Namespace` parameter.

2. **Never put provider code in the framework.** Providers are in `providers/provider-*` modules.

3. **Never bypass NATS.** All node communication MUST flow through NATS subjects. No direct method calls between nodes.

4. **Never skip namespace in REST endpoints.** `?namespace=` is required for all tenant-scoped endpoints.

5. **Never use old terminology.** Use `system` (not the old name), `node` (not the old name), `fallback` (not the old name).

6. **Never write fallbacks.** The platform uses a strict fail-fast approach. If a dependency (NATS, DuckDB, GraalJS) is unavailable, the platform MUST throw an error and refuse to start — NOT silently degrade or fall back to an alternative. Fallbacks are evil and forbidden. They hide problems, create unpredictable behavior, and make debugging impossible. If NATS is down, fail. If DuckDB is corrupted, fail. If a provider throws, let it throw. Every failure should be loud, immediate, and actionable.

   **Specific examples of forbidden fallbacks:**
   - In-memory message bus when NATS is unavailable → NO. Throw.
   - File-based persistence when DuckDB fails → NO. Throw.
   - Default values when config is missing → NO. Throw with a clear message.
   - Catch-and-log instead of catch-and-throw → NO. Let exceptions propagate.
   - Return null/empty when an operation fails → NO. Throw.

---

## Conventions

### Java
- Java 21+. Records for value types. Sealed interfaces for closed hierarchies.
- CDI: `@ApplicationScoped` for singletons. `@Inject` constructor injection.
- Logging: SLF4J. Parameterized messages.
- Tests: JUnit 5 + AssertJ + Mockito + Awaitility.

### Go (CLI)
- Go 1.24+.
- `cmd/` for cobra commands, `internal/client/` for HTTP client, `internal/model/` for DTOs, `internal/output/` for printers.

### TypeScript (user-facing)
- `.quark.ts` files use `` from `quark.d.ts`.
- Users get type safety from the npm package's `.d.ts` files.
- The server transpiles TS → JS before GraalJS evaluation.

---

## Build & test commands

```bash
make build          # Build everything (Java + Go)
make test           # Run all tests (Java + Go)
make verify         # Clean → Build → Test (CI-friendly)
make run-example    # Run the streaming example
make server-dev     # Start Quarkus dev mode (port 8080)
```
