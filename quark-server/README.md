# Quark Control Plane (Go)

The Go control plane is a single binary that owns the REST API,
deploy/undeploy orchestration, and ProcessManager (which spawns
data-plane JVM processes). It has **no TypeScript parsing**, **no
GraalJS**, and **no in-memory node registry** — every node lookup goes
through the Catalog via NATS.

## Build

```bash
make build-server   # produces quark-server/quark-server
```

## Run

```bash
make run-server     # starts on :8080 (override with QUARK_HTTP_PORT=...)
```

## Configuration

All config is via env vars (prefix `QUARK_`):

| Var | Default | Purpose |
|-----|---------|---------|
| `QUARK_HTTP_PORT` | `8080` | REST API port |
| `QUARK_NATS_URL` | `nats://localhost:4222` | NATS broker URL |
| `QUARK_STATE_ROOT` | `./quark-state` | State root (catalog DB, data-plane logs) |
| `QUARK_DATAPLANE_BINARY` | (auto-detect) | Explicit data-plane JAR/native binary path |
| `QUARK_DATAPLANE_PORT_BASE` | `9100` | Starting port for data-plane HTTP servers |
| `QUARK_LOG_FORMAT` | `console` | `console` (dev) or `json` (prod) |
| `QUARK_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Package Layout

```
quark-server/
├── cmd/server/main.go             # entry point + graceful shutdown
└── internal/
    ├── config/                    # env-var config
    ├── domain/                    # Go structs mirroring Java records
    ├── nats/                      # NATS connection wrapper
    ├── store/                     # repository interfaces + NatsCatalogClient
    ├── dataplane/                 # ProcessManager + DataPlaneProcess + ipc
    ├── deploy/                    # DeployService (persist + forward via NATS)
    ├── event/                     # event receiver (quark.data.event.> sub)
    ├── metrics/                   # heartbeat collector + rate computer
    ├── query/                     # read-side services
    ├── health/                    # /health/live + /health/ready
    └── http/                      # Fiber app + handlers + middleware + DTOs
```

## Architectural Rules

- **Layer strictly**: `http/handler → service → store → nats`. Handlers
  never call NATS directly.
- **DI via constructors**: every struct takes its dependencies as
  constructor params. No package-level mutable state. No `init()`.
- **Context propagation**: every I/O function takes `context.Context`
  as the first argument and respects cancellation.
- **Error handling**: return errors, don't panic. Wrap with
  `fmt.Errorf("...: %w", err)` at boundaries.
- **No TypeScript parsing**: the server treats `.quark.ts` as opaque.
  A minimal regex "sniffer" in `internal/deploy/service.go` extracts
  just the system name + runtime mode (shared/isolated) for NATS
  routing. Full parsing happens in the runtime via
  `SimpleSystemParser` + `GraalJsSystemParser`.

## Wire Format Compatibility

The Go server uses the exact same wire format as the deleted Java
server:

- Same NATS subjects (`catalog.*`, `quark.control.*`, `quark.data.*`)
- Same JSON shapes (CLI's `internal/model/*.go` structs unchanged)
- Same health endpoint paths (`/health/live`, `/health/ready`)
- Same REST paths + methods (26 endpoints)

The one known wrinkle: the Java data plane's Jackson serializer emits
`Instant` as epoch-seconds by default. The Go server's
`domain.NodeEvent.TimestampString()` handles both epoch-seconds (the
default) and RFC3339 strings (from the Catalog), auto-detecting via
a magnitude heuristic.

## Tests

```bash
go test ./...                # all packages
go test -v ./internal/store  # verbose, single package
go test -race ./...          # race detector
```

The `store` package's tests start an in-process `nats-server` and
register mock Catalog handlers, exercising the wire format end-to-end.
