# Quark CLI — `quarkctl`

A Go-based command-line interface for the Quark platform, Talks to the Quark server's REST API only — shares no code with the server.

## Design Principles

1. **Conceptual alignment with the server** — every CLI command maps 1:1 to a REST endpoint.
2. **noun-verb command structure** — noun-verb command structure (`quarkctl apply`, `quarkctl pause node cpu`).
3. **`--json` flag on every command** — for AI agents and scripting.
4. **Namespace is required** — every tenant-scoped command requires `--namespace` (or `QUARK_NAMESPACE` env var).
5. **No business logic in the CLI** — the CLI is a thin HTTP client.

## Installation

**Prerequisite**: Go 1.24 or later.

### Via Makefile (recommended)

```bash
make cli           # builds cli/quarkctl
make install-cli   # copies to /usr/local/bin/quarkctl (requires sudo)
```

## Configuration

| Flag | Shorthand | Env var | Default | Description |
|------|-----------|---------|---------|-------------|
| `--host` | | `QUARK_HOST` | `http://localhost:8080` | Server URL |
| `--namespace` | `-n` | `QUARK_NAMESPACE` | (required) | Tenant namespace |
| `--json` | | `QUARK_OUTPUT=json` | `false` | Output raw JSON |
| `--timeout` | | | `30s` | HTTP timeout |

## Command Reference

### `quarkctl system` — manage systems

```bash
# Deploy a system from a .quark.ts file
quarkctl apply -f monitor.quark.ts -n alice

# List systems in a namespace
quarkctl get systems -n alice

# Get system details (node states, health)
quarkctl get system monitor -n alice

# Get the original .quark.ts source
quarkctl system source monitor -n alice

# Undeploy a system
quarkctl delete system monitor -n alice
```

### `quarkctl node` — manage individual nodes

```bash
# List nodes in a namespace (optionally within a specific system)
quarkctl get nodes -n alice
quarkctl get nodes -n alice -s monitor

# Get node details
quarkctl get node cpu -n alice -s monitor

# Lifecycle operations
quarkctl pause node cpu -n alice -s monitor
quarkctl node resume cpu -n alice -s monitor
quarkctl node drain cpu -n alice -s monitor
quarkctl node archive cpu -n alice -s monitor
quarkctl node recover cpu -n alice -s monitor
quarkctl node delete cpu -n alice -s monitor
```

### `quarkctl registry` — browse available node implementations

```bash
quarkctl get registry
quarkctl get registry --category source
quarkctl get registry --query timer
quarkctl registry get source/timer:v1
```

### `quarkctl event` — query the event log

```bash
quarkctl get events -n alice
quarkctl get events -n alice -s monitor
quarkctl get events -n alice --kinds NODE_STATE_CHANGED
quarkctl event count -n alice
quarkctl watch events -n alice -s monitor
```

### `quarkctl health` — platform / namespace / system / node health

```bash
quarkctl get namespaces
quarkctl get namespace alice
quarkctl get system monitor -n alice
quarkctl describe node cpu -n alice -s monitor
```

## Conceptual alignment with the server

| Server REST endpoint | CLI command |
|---------------------|-------------|
| `POST /systems/deploy` | `quarkctl apply -f ... -n ...` |
| `GET /systems?namespace=` | `quarkctl get systems -n ...` |
| `GET /systems/{name}?namespace=` | `quarkctl get system NAME -n ...` |
| `GET /systems/{name}/source?namespace=` | `quarkctl system source NAME -n ...` |
| `DELETE /systems/{name}?namespace=` | `quarkctl delete system NAME -n ...` |
| `GET /nodes?namespace=&system=` | `quarkctl get nodes -n ... [-s ...]` |
| `GET /nodes/{name}?namespace=&system=` | `quarkctl get node NAME -n ... -s ...` |
| `POST /nodes/{name}/pause` | `quarkctl node pause NAME -n ... -s ...` |
| `POST /nodes/{name}/resume` | `quarkctl node resume NAME -n ... -s ...` |
| `POST /nodes/{name}/drain` | `quarkctl node drain NAME -n ... -s ...` |
| `POST /nodes/{name}/archive` | `quarkctl node archive NAME -n ... -s ...` |
| `POST /nodes/{name}/recover` | `quarkctl node recover NAME -n ... -s ...` |
| `POST /nodes/{name}/delete` | `quarkctl node delete NAME -n ... -s ...` |
| `GET /registry` | `quarkctl get registry` |
| `GET /registry/{uri}` | `quarkctl registry get URI` |
| `GET /events?namespace=` | `quarkctl get events -n ...` |
| `GET /events/count?namespace=` | `quarkctl event count -n ...` |
| `GET /health` | `quarkctl get namespaces` |
| `GET /health/nodes/{name}?namespace=&system=` | `quarkctl describe node NAME -n ... -s ...` |
