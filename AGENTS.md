# AGENTS.md — Guide for AI Agents Working on Quark

> **If you read nothing else, read this:** Quark is a **three-service platform** with a strict tier-based directory layout. The `.quark.ts` file IS the program — users write only TypeScript and never touch Java. The flow: CLI sends the TypeScript to the **control plane** (server), which parses it with a regex-based `SimpleSystemParser` (no GraalJS), persists it to the **Catalog** (Go + SQLite, via NATS), and forwards deploy commands to a **data plane** (runtime) process that uses GraalJS to execute TypeScript node logic over an external NATS server.

---

## Quick orientation

```
quark-platform/
├── AGENTS.md                         ← this file — READ FIRST
├── README.md                         ← human-facing project overview
├── Makefile                          ← all build/test/run commands
├── pom.xml                           ← Maven parent POM
├── mvnw, mvnw.cmd, .mvn/            ← Maven wrapper (DO NOT delete .mvn/wrapper/)
│
├── docs/                             ← specifications
│   ├── abstraction.md                ← the Node concept (vision)
│   ├── DESIGN.md                     ← design principles
│   ├── DECLARATION.md                ← the .quark.ts format (syntax reference)
│   ├── NODE.md                       ← the Node spec (full reference)
│   ├── CLI.md                        ← CLI / server conceptual alignment
│   └── USER-STORY.md                 ← how a typical user interacts with the system
│
├── core/                             ← SHARED code — no GraalJS, no Quarkus
│   ├── quark-domain/                 ← Pure domain model (records, sealed interfaces)
│   ├── quark-event/                  ← Event bus + per-tenant event store SPI
│   ├── quark-registry/               ← SPI registry for node implementations
│   ├── quark-script/                 ← SystemParser interface + SimpleSystemParser (regex)
│   └── quark-engine/                 ← Lifecycle, NATS wiring, DataPlaneProcess, ProcessManager
│
├── server/                           ← CONTROL PLANE — no GraalJS
│   ├── quark-app/                    ← DeployService, QueryService, LifecycleService, HealthService,
│   │                                 ← NatsCatalogClient (catalog.* NATS adapter)
│   ├── quark-api/                    ← JAX-RS REST endpoints + DTOs (/api/v1/...)
│   ├── quark-observability/          ← Metrics, health checks
│   └── quark-server/                 ← Quarkus runner (QuarkServer.java, native-image config)
│
├── runtime/                          ← DATA PLANE — includes GraalJS/Truffle
│   ├── quark-script/                 ← GraalJsSystemParser (GraalJS-based parser, runtime-only)
│   ├── quark-polyglot/               ← TypeScriptNodeFactory + GraalJS providers (Source/Function/Store/Endpoint)
│   ├── quark-app/                    ← RuntimeDeployService, DataPlaneCommandHandler, forwarders
│   ├── quark-runtime/                ← Quarkus runner (QuarkRuntime.java, native-image config w/ --macro:truffle-svm)
│   └── providers/                    ← Node implementations (timer, cpu-profiler, etc.)
│       ├── provider-stubs/           ← Noop/memory stubs (testing)
│       ├── provider-timer/           ← source/timer:v1
│       ├── provider-cpu-profiler/    ← function/cpu-profiler:v1
│       ├── provider-memory-profiler/← function/memory-profiler:v1
│       ├── provider-list/            ← store/list:v1
│       ├── provider-json-writer/     ← store/json-writer:v1
│       └── provider-streaming-endpoint/ ← endpoint/stream:v1
│
├── quark-catalog/                    ← CATALOG service (Go + SQLite)
│   ├── cmd/quark-catalog/main.go     ← Entry point: flags + wiring
│   └── internal/
│       ├── config/                   ← Config struct from flags
│       ├── natsx/                    ← NATS connection w/ retry
│       ├── api/                      ← JSON request/response types per domain
│       ├── store/                    ← SQLite persistence (modernc.org/sqlite, pure Go)
│       └── server/                   ← NATS handlers (catalog.* + registry.*)
│
├── cli/                              ← Go CLI (quarkctl)
│   ├── main.go
│   ├── cmd/                          ← Cobra commands
│   └── internal/                     ← HTTP client + model + output printers
│
└── example/
    └── simple-streaming/             ← Multi-tenant streaming monitor example
        ├── README.md
        ├── system.quark.ts           ← The "program" — this is ALL the user writes
        └── json/                     ← Output directory (server writes here)
```

---

## Architecture

### Three-service model

Quark runs as **three cooperating services** plus an external NATS broker:

| Service | Language | Binary | Includes GraalJS? | Role |
|---------|----------|--------|-------------------|------|
| **Control plane** (server) | Java/Native | `server/quark-server/target/quark-server-0.1.0-SNAPSHOT-runner` (76 MB native, 4 min build) | ❌ No | REST API, deploy/undeploy orchestration, spawns data-plane processes |
| **Catalog** | Go | `quark-catalog/quark-catalog` (15 MB) | ❌ No | SQLite-backed metadata store (systems, nodes, events, sources, registry) |
| **Data plane** (runtime) | Java/Native | `runtime/quark-runtime/target/quark-runtime-runner-runner` (194 MB native, 9 min build) | ✅ Yes (via `--macro:truffle-svm`) | Executes nodes, hosts GraalJS, runs providers |
| **NATS broker** | Go | `nats-server` (external) | n/a | Message bus for all inter-service communication |

The data plane is **spawned on demand** by the control plane's `ProcessManager`. There is one shared runtime process (`runtimeId=shared`) for non-isolated namespaces; isolated namespaces get their own process (`runtimeId=ns-<namespace>`).

### Tier-based code separation

Three Maven tiers under the workspace root, each with strict dependency rules:

| Tier | Modules | May depend on | May NOT depend on |
|------|---------|---------------|-------------------|
| **core/** | quark-domain, quark-event, quark-registry, quark-script, quark-engine | each other, no GraalJS, no Quarkus | GraalJS, Quarkus, server/, runtime/ |
| **server/** | quark-app, quark-api, quark-observability, quark-server | core/, Quarkus, NATS | GraalJS/Truffle, runtime/ |
| **runtime/** | quark-script, quark-polyglot, quark-app, quark-runtime, providers/* | core/, Quarkus, NATS, GraalJS/Truffle | server/ |

**Key invariant:** `core/quark-script` contains only `SimpleSystemParser` (regex-based, no GraalJS). The GraalJS-based `GraalJsSystemParser` lives in `runtime/quark-script`. This is what allows the server native image to stay small (76 MB vs 194 MB for the runtime).

### NATS subjects

Two distinct subject taxonomies flow through NATS:

**Control-plane ↔ data-plane IPC** (server ↔ runtime):
- `quark.control.<runtimeId>.deploy` — deploy command (JSON)
- `quark.control.<runtimeId>.undeploy` — undeploy command (JSON)
- `quark.data.<runtimeId>.status` — status response (JSON)
- `quark.data.event.>` — events forwarded from data plane to control plane (wildcard subscription)
- `quark.data.heartbeat.>` — per-namespace metrics heartbeats (wildcard)

**Control-plane ↔ Catalog** (server ↔ catalog):
- `catalog.system.{save,get,list,delete,updateState}` — system metadata
- `catalog.node.{save,saveAll,list,delete}` — node metadata
- `catalog.event.{append,appendBatch,query,count}` — event log
- `catalog.source.{save,get,list}` — `.quark.ts` source storage
- `catalog.registry.{save,find,list,exists}` — built-in node descriptors
- `registry.node.{push,pull,info,list,search,delete,exists}` — node package registry

**Node-data subjects** (runtime-internal, between nodes):
- `<namespace>.<system>.<node>.<event>` — node-to-node events (e.g. `alice.monitor.timer.tick`)
- Note: as of v8, NATS **Core** is used (not JetStream) — no message persistence, no automatic retries, no fallback routing. The `onFailure` field in `.quark.ts` is parsed but **not enforced** at runtime. See `docs/NODE.md` §6.

---

## Common pitfalls

0. **Never create standalone runners or require users to write Java code.** The `.quark.ts` file IS the program. Users deploy via `quarkctl apply -f file.quark.ts -n alice`. The control plane is the interpreter (parser + orchestrator); the data plane is the executor (GraalJS + providers).

1. **Never add cross-namespace methods.** All lookups require a `Namespace` parameter.

2. **Never put provider code in the framework.** Providers live in `runtime/providers/provider-*`.

3. **Never bypass NATS.** All node-to-node communication flows through NATS subjects. The control plane talks to the data plane via NATS, not via in-process method calls.

4. **Never skip namespace in REST endpoints.** `/api/v1/namespaces/{ns}/...` is required for all tenant-scoped endpoints.

5. **Never use stale terminology.** Use `system`, `node`, `onFailure.routeTo` (the domain concept of a fallback subject). Avoid "fallback" as a verb — see pitfall 6.

6. **Never write code-level fallbacks.** The platform uses a strict fail-fast approach. If NATS is down, fail. If the Catalog is corrupted, fail. If a provider throws, let it throw. **This is about engineering resilience, not the `onFailure` domain concept** — `onFailure.routeTo` in `.quark.ts` defines a fallback *subject* (a valid domain modeling primitive), but the runtime does not silently retry or degrade; it routes the event to the named subject and lets the user's nodes handle it.

   **Specific examples of forbidden code-level fallbacks:**
   - In-memory message bus when NATS is unavailable → NO. Throw.
   - File-based persistence when the Catalog fails → NO. Throw.
   - Default values when config is missing → NO. Throw with a clear message.
   - Catch-and-log instead of catch-and-throw → NO. Let exceptions propagate.

7. **Never delete `.mvn/wrapper/`.** The `mvnw` script needs `.mvn/wrapper/maven-wrapper.jar` to bootstrap Maven. Excluding this directory from a zip/source distribution breaks `make build`.

8. **Never confuse the two runner jar variants.** Quarkus produces TWO jars per module:
   - `<finalName>.jar` — thin jar, **no main manifest**, NOT runnable
   - `<finalName>-runner.jar` — fat jar, runnable
   
   With `finalName=quark-runtime-runner`, the runnable jar is `quark-runtime-runner-runner.jar` (yes, double `-runner` suffix). `ProcessManager.findBinary()` and any packaging scripts must use the `-runner` variant.

9. **Native image: server ≠ runtime.** The server native image must NOT include `--macro:truffle-svm` (keeps it at 76 MB, 3 GB peak RAM). The runtime native image MUST include it (194 MB, 6.5 GB peak RAM). The server uses `SimpleSystemParser` (regex); the runtime uses `GraalJsSystemParser` + `TypeScriptNodeFactory` (GraalJS).

10. **GraalJS enterprise is broken in native image.** `org.graalvm.polyglot:js` (enterprise) pulls in `truffle-enterprise`, whose `HSPathGen$EndPoint` calls a non-existent `Path.resolve(String, String[])` overload — a known GraalVM 21.0.11 + Truffle 24.1.x incompatibility. Use `org.graalvm.js:js-community` instead (community edition, no `truffle-enterprise`, sufficient for TS execution).

---

## Conventions

### Java
- Java 21+. Records for value types. Sealed interfaces for closed hierarchies.
- CDI: `@ApplicationScoped` for singletons. `@Inject` constructor injection.
- Logging: SLF4J. Parameterized messages.
- Tests: JUnit 5 + AssertJ + Mockito + Awaitility.

### Go (CLI and Catalog)
- Go 1.24+.
- CLI layout: `cmd/` for cobra commands, `internal/client/` for HTTP client, `internal/model/` for DTOs, `internal/output/` for printers.
- Catalog layout: `cmd/quark-catalog/main.go` for entry point, `internal/{config,natsx,api,store,server}/` for logic. Each package has a single responsibility (config = flag parsing, natsx = connection, api = wire types, store = SQL only, server = NATS handlers).
- SQLite driver: `modernc.org/sqlite` (pure Go, no CGO) — required for trivial portability of the Catalog binary.

### TypeScript (user-facing)
- `.quark.ts` files use `defineSystem()` from `quark.d.ts`.
- Users get type safety from the npm package's `.d.ts` files.
- The control plane parses with `SimpleSystemParser` (regex). The data plane evaluates TypeScript node logic via GraalJS.

---

## Build & test commands

```bash
# JVM builds (default)
make build              # All Java modules + Go CLI + Go Catalog
make build-java         # Java modules only
make build-go           # Go CLI only
make build-catalog      # Go Catalog only
make test               # Java + Go tests
make verify             # Clean → Build → Test (CI-friendly)

# Native builds (require Oracle GraalVM 21+ with native-image)
make build-native           # Both server + runtime native binaries (~13 min total)
make build-native-server    # Control plane only (~4 min, 3 GB RAM, 76 MB output)
make build-native-runtime   # Data plane with GraalJS (~9 min, 6.5 GB RAM, 194 MB output)

# Run
make run-server             # JVM mode control plane
make run-server-native      # Native mode control plane
make run-example            # Deploy + observe the streaming example (15s)
make run-example RUN_MODE=native  # Same but with native binaries

# Dev
make server-dev             # Quarkus dev mode (hot reload, port 8080)
make clean                  # Remove all build artifacts
make clean-native           # Remove only native binaries
```

The `RUN_MODE=jvm|native` env var (replaces the old `BUILD_MODE`) controls which binary `make run-*` targets use. Native builds are independent of `RUN_MODE` — `make build-native-*` always builds native regardless.
