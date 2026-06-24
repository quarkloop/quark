# Quark CLI — `quarkctl`

A Go-based command-line interface for the Quark platform. Talks to the **control plane's REST API only** — shares no code with the server, the Catalog, or the data plane.

## Architecture context

The CLI is one of four clients of the control plane (the others being the Catalog, the data plane, and the NATS broker itself). The flow is always:

```
quarkctl → control plane (REST /api/v1/*) → Catalog (NATS catalog.*) for persistence
                                      → data plane (NATS quark.control.*.deploy) for execution
```

The CLI never talks to the Catalog or the data plane directly. Every CLI command maps 1:1 to a REST endpoint on the control plane.

## Design Principles

1. **Conceptual alignment with the control plane** — every CLI command maps 1:1 to a REST endpoint under `/api/v1/`.
2. **noun-verb command structure** — `quarkctl apply`, `quarkctl get systems`, `quarkctl delete system`.
3. **`--json` flag on every command** — for AI agents and scripting.
4. **Namespace is required** — every tenant-scoped command requires `--namespace` / `-n` (or `QUARK_NAMESPACE` env var).
5. **No business logic in the CLI** — the CLI is a thin HTTP client.

## Installation

**Prerequisite**: Go 1.24 or later.

### Via Makefile (recommended)

```bash
make build-go      # builds cli/quarkctl
```

The binary is at `cli/quarkctl`. Copy it to your `$PATH` manually if you want it globally available (the Makefile does not install to `/usr/local/bin`).

## Configuration

| Flag | Shorthand | Env var | Default | Description |
|------|-----------|---------|---------|-------------|
| `--host` | | `QUARK_HOST` | `http://localhost:8080` | Control plane URL |
| `--namespace` | `-n` | `QUARK_NAMESPACE` | (required) | Tenant namespace |
| `--json` | | `QUARK_OUTPUT=json` | `false` | Output raw JSON |
| `--timeout` | | | `30s` | HTTP timeout |

## Command Reference

### Systems — deploy, list, inspect, delete

```bash
# Deploy a system from a .quark.ts file
quarkctl apply -f monitor.quark.ts -n alice

# List systems in a namespace
quarkctl get systems -n alice

# Get system details (node states, health)
quarkctl get system monitor -n alice

# Undeploy a system
quarkctl delete system monitor -n alice
```

### Nodes — list and inspect

```bash
# List nodes in a namespace (optionally within a specific system)
quarkctl get nodes -n alice
quarkctl get nodes -n alice -s monitor

# Get node details
quarkctl get node cpu -n alice -s monitor
```

### Node Package Registry — push, pull, search

The `node` subcommand manages the node package registry (pushed `.ts`/`.so` payloads stored in the Catalog):

```bash
quarkctl node list                          # list all registered node packages
quarkctl node list --category source        # filter by category
quarkctl node info source/timer:v1          # show metadata for a package
quarkctl node search timer                  # search by keyword in URI/manifest
quarkctl node push -f my-node.ts --uri source/my-node:v1
quarkctl node pull source/timer:v1          # download a package's content
```

### Events — query and watch

```bash
quarkctl get events -n alice                # recent events in alice namespace
quarkctl get events -n alice -s monitor     # filter to a specific system
quarkctl watch events -n alice              # tail events live (Ctrl+C to stop)
```

### Namespaces and Health

```bash
quarkctl get namespaces                     # list all active namespaces
```

## Conceptual alignment with the control plane

| Control plane REST endpoint | CLI command |
|-----------------------------|-------------|
| `PUT /api/v1/namespaces/{ns}/systems/{name}` | `quarkctl apply -f ... -n ...` |
| `GET /api/v1/namespaces/{ns}/systems` | `quarkctl get systems -n ...` |
| `GET /api/v1/namespaces/{ns}/systems/{name}` | `quarkctl get system NAME -n ...` |
| `DELETE /api/v1/namespaces/{ns}/systems/{name}` | `quarkctl delete system NAME -n ...` |
| `GET /api/v1/namespaces/{ns}/systems/{name}/source` | (no CLI command yet — use curl) |
| `GET /api/v1/namespaces/{ns}/systems/{name}/nodes` | `quarkctl get nodes -n ... -s ...` |
| `GET /api/v1/namespaces/{ns}/systems/{name}/nodes/{node}` | `quarkctl get node NAME -n ... -s ...` |
| `GET /api/v1/namespaces/{ns}/events` | `quarkctl get events -n ...` |
| `GET /api/v1/namespaces` | `quarkctl get namespaces` |
| `GET /api/v1/registry` | `quarkctl node list` |

## Building the CLI

```bash
make build-go          # build cli/quarkctl
make build             # build Java + Go CLI + Catalog (full build)
```

See `AGENTS.md` for the full Makefile target reference.
