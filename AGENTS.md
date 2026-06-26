# Agent Guide

Quark is a **three-service platform** with a strict service-based directory layout. The `.quark.ts` file IS the program — users write only TypeScript and never touch Java. The flow: CLI sends the TypeScript to the **control plane** (server, Go + Fiber), which persists it verbatim to the **Catalog** (Go + SQLite, via NATS) and forwards deploy commands to a **data plane** (runtime, Java/GraalJS) process. The data plane parses the source with `GraalJsSystemParser` (full ESM evaluation) + `SimpleSystemParser` (structural extraction), pulls every node package from the Catalog via `registry.node.pull`, and executes TypeScript node logic over an external NATS server.

## Repository

- **Name**: Quark Platform
- **Language**: Java 21+ (Quarkus/GraalVM), Go 1.24+ (Fiber/nats.go), TypeScript (node definitions)
- **License**: Apache 2.0
- **Repo**: [github.com/quarkloop/quark](https://github.com/quarkloop/quark)
- **Guidelines**: [quarkloop/guidelines](https://github.com/quarkloop/guidelines)

## Quick reference

```bash
# Build everything (JVM mode)
make build              # Java modules + Go CLI + Go Catalog + Go control plane

# Build native executables
make build-native       # Both native binaries + Go CLI + Catalog
make build-native-server    # Control plane only (~4 min)
make build-native-runtime   # Data plane with GraalJS (~9 min)

# Run the example
make run-example        # JVM mode, 15-second run
make run-example RUN_MODE=native  # Native mode

# Dev
make server-dev         # Quarkus dev mode (hot reload, port 8080)
make clean              # Remove all build artifacts
make clean-native       # Remove only native binaries
```

---

## Structure

```
quark-platform/
├── AGENTS.md                         ← this file — READ FIRST
├── README.md                         ← human-facing project overview
├── Makefile                          ← all build/test/run commands
├── pom.xml                           ← Maven parent POM (quark-runtime/* modules only)
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
├── quark-server/                           ← CONTROL PLANE — Go + Fiber (single binary)
│   ├── go.mod, go.sum                ← module github.com/quarkloop/quark/server
│   ├── cmd/server/main.go            ← entry point: env config + graceful shutdown
│   └── internal/
│       ├── config/                   ← env-var config (QUARK_HTTP_PORT, etc.)
│       ├── domain/                   ← Go structs mirroring Java records
│       ├── nats/                     ← NATS connection wrapper (reconnect logic)
│       ├── store/                    ← repository interfaces + NatsCatalogClient
│       │                             ←   (implements all 5 interfaces via NATS)
│       ├── dataplane/                ← ProcessManager + DataPlaneProcess + ipc
│       │                             ←   (spawns JVM data-plane processes)
│       ├── deploy/                   ← DeployService (persist + forward via NATS;
│       │                             ←   NO TypeScript parsing — just a minimal
│       │                             ←   regex "sniffer" for system name +
│       │                             ←   runtime mode)
│       ├── event/                    ← event receiver (quark.data.event.> sub)
│       ├── metrics/                  ← heartbeat collector + rate computer
│       ├── query/                    ← read-side services (System/Node/Namespace/
│       │                             ←   Event/Source/Registry/Lifecycle queries)
│       ├── health/                   ← /health/live + /health/ready
│       └── http/                     ← Fiber app + handlers + middleware + DTOs
│       (NO TypeScript parsing, NO GraalJS, NO in-memory node registry)
│
├── quark-runtime/                          ← DATA PLANE — Java + GraalJS/Truffle
│   ├── quark-core/                   ← Consolidated module: domain records, engine
│   │                                 ← (NATS, lifecycle, dataplane, metrics, polyglot
│   │                                 ← lookup, store SPIs), event bus, registry SPI,
│   │                                 ← and SimpleSystemParser. (Pre-v6 these were
│   │                                 ← split across 5 modules in core/.)
│   ├── quark-script/                 ← GraalJsSystemParser (GraalJS ESM-based parser)
│   ├── quark-polyglot/               ← TypeScriptNodeFactory + PolyglotNodeRegistry
│   │                                 ←   (catalog pull) + JsConsole/JsConfig/JsMessage/
│   │                                 ←   JsPublisher bridges
│   ├── quark-app/                    ← RuntimeDeployService, DataPlaneCommandHandler,
│   │                                 ←   forwarders
│   └── quark-runtime/                ← Quarkus runner (QuarkRuntime.java,
│                                     ←   native-image config w/ --macro:truffle-svm)
│   (NO providers/ subdir — runtime pulls every node from the Catalog at deploy time)
│
├── quark-nodes/                            ← STANDARD LIBRARY (canonical node source)
│   ├── README.md                     ← node layout + the 18 domains
│   ├── CHECKLIST.md                  ← 9-phase node implementation checklist
│   └── quark/                        ← namespace
│       ├── time/schedule/timer/v1/   ← Java node: emits tick events
│       ├── system/cpu/profile/v1/    ← Java node: CPU profiler
│       ├── system/memory/profile/v1/ ← Java node: memory profiler
│       ├── io/file/write/v1/         ← Java node: JSONL file writer
│       ├── stream/sse/broadcast/v1/  ← Java node: SSE broadcast endpoint
│       ├── log/console/stdout/v1/    ← TypeScript node: stdout JSON logger
│       ├── codec/json/parse/v1/      ← TypeScript node: JSON parser
│       ├── data/shape/map/v1/        ← TypeScript node: field mapper
│       ├── route/flow/conditional/v1/← TypeScript node: conditional router
│       └── net/http/fetch/v1/        ← TypeScript node: HTTP fetcher
│   Each node dir contains: manifest.json, src/node.{java,ts}, build.toml, README.md
│   Build + push flow: quarkctl node build <uri> → quarkctl node push <uri>
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
├── quark-cli/                              ← Go CLI (quarkctl)
│   ├── main.go
│   ├── cmd/                          ← Cobra commands
│   └── internal/                     ← HTTP client + model + output printers
│
└── example/
    ├── simple-streaming/             ← Multi-tenant streaming monitor example
    │   ├── README.md
    │   ├── system.quark.ts           ← The "program" — this is ALL the user writes
    │   └── json/                     ← Output directory (server writes here)
    ├── json-pipeline/                ← Timer → JSON parse → map → stdout pipeline
    └── conditional-routing/          ← Conditional router with two stdout destinations
```

---

## Rules

### Three-service model

Quark runs as **three cooperating services** plus an external NATS broker:

| Service | Language | Binary | Includes GraalJS? | Role |
|---------|----------|--------|-------------------|------|
| **Control plane** (quark-server) | Go | `quark-server/quark-server` (~13 MB Go binary, <5s build) | ❌ No | REST API, deploy/undeploy orchestration, spawns data-plane processes |
| **Catalog** | Go | `quark-catalog/quark-catalog` (15 MB) | ❌ No | SQLite-backed metadata store (systems, nodes, events, sources, registry) |
| **Data plane** (quark-runtime) | Java/Native | `quark-runtime/quark-runtime/target/quark-runtime-runner-runner` (194 MB native, 9 min build) | ✅ Yes (via `--macro:truffle-svm`) | Executes nodes, hosts GraalJS, runs providers |
| **NATS broker** | Go | `nats-server` (external) | n/a | Message bus for all inter-service communication |

The data plane is **spawned on demand** by the control plane's `ProcessManager`. There is one shared runtime process (`runtimeId=shared`) for non-isolated namespaces; isolated namespaces get their own process (`runtimeId=ns-<namespace>`).

### Service-based code separation

The platform is split into four top-level trees, each with strict dependency rules. There is **no shared Java source** between the server (Go) and the runtime (Java) — the stable contract between them is the JSON wire format over NATS.

| Tree | Language | May depend on | May NOT depend on |
|------|----------|---------------|-------------------|
| **quark-server/** | Go (Fiber + nats.go + zap) | stdlib + Fiber + nats.go + zap + envconfig | Java, GraalJS, quark-runtime/, core/ (doesn't exist) |
| **quark-runtime/** | Java (Quarkus + GraalJS) | quark-core (internal), Quarkus, NATS, GraalJS/Truffle | server/, quark-nodes/ (runtime pulls nodes from the Catalog, never compiles them) |
| **quark-catalog/** | Go (SQLite + nats.go) | stdlib + modernc.org/sqlite + nats.go | Java, GraalJS, quark-server/, quark-runtime/ |
| **quark-cli/** | Go (Cobra) | stdlib + cobra | Java, GraalJS, quark-server/, quark-runtime/ (talks to the server via HTTP only) |

**Key invariant:** `quark-runtime/quark-core/.../script/SimpleSystemParser` is a comment-aware regex parser (no GraalJS). The GraalJS-based `GraalJsSystemParser` lives in `quark-runtime/quark-script/`. The Go control plane has **no parser at all** — it treats `.quark.ts` as an opaque string and forwards it verbatim to the runtime via NATS. A minimal regex "sniffer" in `quark-server/internal/deploy/service.go` extracts just the system name and runtime mode (shared/isolated) for NATS routing; full parsing happens in the runtime.

**TypeScript handling:** GraalJS Community Edition does NOT natively parse TypeScript (see [graaljs#784](https://github.com/oracle/graaljs/issues/784)). The platform's `.ts` files are valid ECMAScript modules using `export default { ... }` without actual type annotations, so the runtime evaluates them directly via GraalJS's native ESM module support (`Source.mimeType("application/javascript+module")` + `js.esm-eval-returns-exports=true`). If real TS type annotations need to be supported in the future, integrate `tsc`/`esbuild`/`swc` at catalog push time.

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

## Boundaries

The platform uses a **docker-image model** for nodes. The runtime
binary NEVER contains node implementations — every node is fetched
from the Catalog on first use and cached for the rest of the process
lifetime. Adding a new node does NOT require rebuilding the runtime;
it only requires pushing the new node package to the Catalog.

### The four phases

```
   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
   │  quark-nodes/     │     │  Catalog    │     │  Catalog    │     │  Runtime    │
   │  (source)   │     │  (registry) │     │  (store)    │     │  (exec)     │
   └──────┬──────┘     └──────┬──────┘     └──────┬──────┘     └──────┬──────┘
          │                   │                   │                   │
          │  quarkctl build   │                   │                   │
          │──────────────────▶│                   │                   │
          │  (javac → .jar)   │                   │                   │
          │                   │                   │                   │
          │  quarkctl push    │                   │                   │
          │──────────────────▶│  registry.node.push (NATS)            │
          │                   │──────────────────▶│                   │
          │                   │  store in SQLite  │                   │
          │                   │                   │                   │
          │                   │                   │  quarkctl apply   │
          │                   │                   │──────────────────▶│
          │                   │                   │                   │ deploy cmd
          │                   │                   │                   │ via NATS
          │                   │                   │                   │
          │                   │  registry.node.pull (NATS)            │
          │                   │◀──────────────────────────────────────│
          │                   │──────────────────▶│                   │
          │                   │  return zip blob  │                   │
          │                   │                   │──────────────────▶│
          │                   │                   │                   │ unzip + load
          │                   │                   │                   │ (URLClassLoader
          │                   │                   │                   │  or GraalJS ESM)
          │                   │                   │                   │
          │                   │                   │                   │ execute node
   └─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
```

### Phase 1: Build (developer machine)

```bash
quarkctl node build quark/time/schedule/timer:v1
```

- Resolves `quark-nodes/quark/time/schedule/timer/v1/` from the URI.
- Reads `manifest.json` to determine the language (`java` | `typescript`).
- For **Java**: resolves the classpath from `manifest.json`'s
  `dependencies.java` list (Maven coordinates → `~/.m2/repository/`),
  runs `javac` to compile `src/*.java` → `target/classes/`, then
  `jar cf` to package → `target/<node>-v<version>.jar`.
- For **TypeScript**: no-op (the runtime evaluates `src/` directly
  via GraalJS ESM; no compile step).

### Phase 2: Push (developer machine → Catalog)

```bash
quarkctl node push quark/time/schedule/timer:v1
```

- Packages `manifest.json` + the build output (`.jar` or `.ts`)
  into a single zip blob.
- POSTs `{ uri, version, manifest, content (zip bytes base64),
  contentType }` to `/api/v1/registry/nodes` on the control plane.
- The control plane forwards to the Catalog via the
  `registry.node.push` NATS subject.
- The Catalog stores `{ uri, version, manifest, content, contentType,
  checksum }` in the `node_packages` SQLite table.

### Phase 3: Pull (runtime → Catalog, on deploy)

When `quarkctl apply -f system.quark.ts -n alice` triggers a deploy:

1. Control plane parses the `.quark.ts`, persists the system record,
   and sends a `DeployCommand` to the data plane via
   `quark.control.<runtimeId>.deploy` (NATS request-reply).
2. The data plane's `SystemDeployer` walks every node in the system
   definition. For each node URI:
   - First checks the in-process `NodeRegistry` (for built-in
     descriptors — usually empty in production).
   - If not found, calls `PolyglotNodeRegistry.lookupFactory(uri)`,
     which sends `registry.node.pull` to the Catalog via NATS.
   - The Catalog returns the zip blob; the runtime unzips it and
     extracts the `.jar` (Java) or `.ts` (TypeScript).
3. For **Java** (`contentType=shared-library`):
   - Materialise the jar to a temp file.
   - Create a `URLClassLoader` with the runtime's classloader as
     parent (so the loaded Factory can see
     `com.quarkloop.quark.runtime.*`).
   - Walk every top-level `.class` in the jar, load each via
     `Class.forName(name, false, loader)`, and check whether it
     implements `NodeImplementationFactory`.
   - Instantiate the first match via `getDeclaredConstructor()` +
     `setAccessible(true)` (quark-nodes/ convention uses package-private
     classes so the file can be named `node.java`).
4. For **TypeScript** (`contentType=typescript`):
   - Pass the source to `TypeScriptNodeFactory`, which evaluates it
     via GraalJS's native ESM module support
     (`Source.mimeType("application/javascript+module")` +
     `js.esm-eval-returns-exports=true`).
5. The loaded `NodeImplementationFactory` is cached by URI for the
   rest of the process lifetime. Subsequent deploys of the same URI
   do NOT re-pull.

The data plane logs every pull at INFO level:
`Loaded node <uri> from catalog (type=<type>, <n> bytes)`.

### Phase 4: Run (runtime executes the node)

Once the factory is loaded, the engine:
- Calls `factory.create(config)` to get a `NodeProvider` instance.
- Calls `provider.init(config)`.
- If the node has `listens`: creates NATS subscriptions that dispatch
  to `provider.onMessage(msg, publisher)`.
- If the node has no `listens` (autonomous): calls
  `provider.start(publisher, config)`.
- On undeploy: calls `provider.close()`.

### What this means in practice

- **Adding a node**: write `quark-nodes/<ns>/<domain>/<subdomain>/<node>/<version>/`,
  run `quarkctl node build <uri>` + `quarkctl node push <uri>`. No
  runtime rebuild.
- **Updating a node**: edit `src/`, re-run `build` + `push`. The
  runtime picks up the new version on the next deploy (the in-process
  cache is per-process; restart the data plane to clear it).
- **Runtime binary**: stays small and generic. It contains the
  engine, the GraalJS/Truffle runtime, and the catalog-pull
  machinery — nothing else. The same binary can run any node that
  has been pushed to the Catalog.

---

## Common mistakes to avoid

0. **Never create standalone runners or require users to write Java code.** The `.quark.ts` file IS the program. Users deploy via `quarkctl apply -f file.quark.ts -n alice`. The control plane is the interpreter (parser + orchestrator); the data plane is the executor (GraalJS + providers).

1. **Never add cross-namespace methods.** All lookups require a `Namespace` parameter.

2. **Never put provider code in the runtime.** Node implementations live exclusively in `quark-nodes/quark/<domain>/<subdomain>/<node>/<version>/src/`. The runtime NEVER compiles them — it pulls every node from the Catalog at deploy time via `registry.node.pull` over NATS and loads it dynamically (TypeScript via GraalJS ESM, Java shared-libraries via URLClassLoader). This is the docker-image model: `quark-nodes/` is the source, the Catalog is the registry, the runtime is the container runtime. Adding a node requires `quarkctl node build <uri>` + `quarkctl node push <uri>` — no runtime rebuild.

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

9. **Native image: server ≠ runtime.** The server native image must NOT include `--macro:truffle-svm` (keeps it at 76 MB, 3 GB peak RAM). The runtime native image MUST include it (194 MB, 6.5 GB peak RAM). The server uses `SimpleSystemParser` (comment-aware regex, no GraalJS); the runtime uses `GraalJsSystemParser` + `TypeScriptNodeFactory` (GraalJS ESM).

10. **GraalJS enterprise is broken in native image.** `org.graalvm.polyglot:js` (enterprise) pulls in `truffle-enterprise`, whose `HSPathGen$EndPoint` calls a non-existent `Path.resolve(String, String[])` overload — a known GraalVM 21.0.11 + Truffle 24.1.x incompatibility. Use `org.graalvm.js:js-community` instead (community edition, no `truffle-enterprise`, sufficient for TS execution).

---

## Commit conventions

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
- `.quark.ts` files use `export default { name, namespace, nodes: { ... } }` syntax.
- The control plane parses with `SimpleSystemParser` (comment-aware, no GraalJS).
- The data plane evaluates TypeScript node logic via GraalJS ESM module support (`js.esm-eval-returns-exports=true`).

---

## Testing

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

## When you're stuck

- Read `docs/architecture.mdx` for the three-service model, process types, and runtime isolation.
- Read `docs/protocol.mdx` for NATS subjects and wire protocol shapes.
- Read `docs/build.mdx` for JVM vs native mode, Makefile targets, and Docker verification.
- Read the [AGENTS.md spec](https://github.com/quarkloop/guidelines/blob/main/agents/SPEC.md) for org-wide conventions.
- Search existing issues and PRs before asking.
- If unsure about a service boundary change, open an issue and ask before implementing.
