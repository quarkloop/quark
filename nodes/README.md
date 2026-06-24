# Quark Standard Library Nodes

This directory contains the Quark standard library — the official node
implementations for the `quark/` namespace.

## Structure

Every node lives at a directory path matching its URI exactly:

```
nodes/
└── quark/                           ← namespace
    └── io/                          ← domain
        └── file/                    ← subdomain
            └── watch/               ← node
                └── v1/              ← version
                    ├── manifest.json
                    ├── src/
                    │   └── node.java
                    ├── build.toml
                    └── README.md
```

The URI `quark/io/file/watch:v1` maps to the path
`nodes/quark/io/file/watch/v1/`. The path IS the URI.

## The 18 Domains

| Domain | Covers |
|--------|--------|
| `time` | Scheduling (timer, cron) |
| `net` | Network protocols (HTTP, gRPC, MQTT, WebSocket) |
| `io` | Filesystem and stream I/O |
| `db` | Databases (Postgres, MySQL, MongoDB, Redis) |
| `data` | Data manipulation (map, filter, validate) |
| `codec` | Encoding/decoding (JSON, CSV, XML, base64) |
| `text` | Text processing (template, extract, analyze) |
| `ai` | AI/ML (inference, embeddings, training) |
| `notify` | Notifications (email, Slack, Discord, push, SMS) |
| `log` | Structured logging (console, syslog, remote) |
| `search` | Search operations (web, vector, fulltext) |
| `stream` | Real-time streaming to clients (SSE, WebSocket) |
| `route` | Flow control (conditional, fan-out, throttle) |
| `compute` | Long-running tasks (transcode, report, batch) |
| `cloud` | Cloud provider services (AWS, GCP, Azure) |
| `system` | System metrics (CPU, memory, disk, process) |
| `console` | Terminal/console (TTY, PTY, CLI) |
| `crypto` | Cryptography (hash, encrypt, sign, verify) |

See the [Node URI Specification](../docs/content/docs/node-uri.mdx) for the
full URI format and the [Node Catalog](../docs/content/docs/node-catalog.mdx)
for the complete list of 114 planned standard library nodes.

## Node Internal Layout

```
quark/<domain>/<subdomain>/<node>/<version>/
├── manifest.json            ← node identity + metadata
├── src/                     ← ALL source code (nothing else goes here)
│   ├── node.java            ← Java implementation (or node.ts for TypeScript)
│   ├── config.java          ← typed config record (Java, optional)
│   └── node.test.java       ← unit tests (optional)
├── build.toml               ← build + package configuration
└── README.md                ← developer documentation
```

**Code and configuration are strictly separated.** Source code lives under
`src/`. Configuration files (`manifest.json`, `build.toml`, `README.md`)
live at the root. Nothing is mixed.

See the [Node Layout Specification](../docs/content/docs/node-layout.mdx)
for the full spec.

## Polyglot Support

Nodes can be implemented in:

- **Java** — for system-level, high-performance, or vendor-SDK-dependent
  nodes. Compiled to `.jar` (JVM mode) and `.so` (native image mode via
  GraalVM `native-image --shared`).
- **TypeScript** — for logic-heavy, API-calling, or rapid-iteration nodes.
  Executed via GraalJS in both JVM and native image modes.

The language is declared in `manifest.json` (`"language": "java"` or
`"language": "typescript"`). The build tool compiles/packages accordingly.

## Build → Package → Push Flow

1. **Build** — `quarkctl node build <uri>` compiles `src/` per `build.toml`.
2. **Package** — zips `manifest.json` + build output into a single blob.
3. **Push** — `quarkctl node push <uri>` sends the zip to the Catalog via NATS.
4. **Pull** — the data plane fetches the node on deploy, unzips, and loads it.

The Catalog stores `{ uri, manifest, content (zip blob), contentType }`.
The data plane loads the content based on runtime mode:
- JVM + `contentType=shared-library` → loads `.jar` via classloader
- Native + `contentType=shared-library` → loads `.so` via `System.load()`
- `contentType=typescript` → evaluates `.ts` via GraalJS

## Implementing a Node

See **[CHECKLIST.md](./CHECKLIST.md)** for the detailed step-by-step
checklist for creating, implementing, building, and pushing a node.

## First 10 Nodes

### Existing nodes (refactor to new layout)

| Current URI | New URI | Language |
|-------------|---------|----------|
| `source/timer:v1` | `quark/time/schedule/timer:v1` | Java |
| `function/cpu-profiler:v1` | `quark/system/cpu/profile:v1` | Java |
| `function/memory-profiler:v1` | `quark/system/memory/profile:v1` | Java |
| `store/json-writer:v1` | `quark/io/file/write:v1` | Java |
| `endpoint/stream:v1` | `quark/stream/sse/broadcast:v1` | Java |

### New nodes to implement

| URI | Language |
|-----|----------|
| `quark/log/console/stdout:v1` | TypeScript |
| `quark/codec/json/parse:v1` | TypeScript |
| `quark/data/shape/map:v1` | TypeScript |
| `quark/route/flow/conditional:v1` | TypeScript |
| `quark/net/http/fetch:v1` | TypeScript |
