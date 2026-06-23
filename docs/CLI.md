# Quark CLI — `quarkctl`

A Go-based command-line interface for the Quark platform, Talks to the Quark server's REST API only — shares no code with the server.

## Design Principles

1. **Conceptual alignment with the server** — every CLI command maps 1:1 to a REST endpoint.
2. **noun-verb command structure** — noun-verb command structure (`quarkctl system deploy`, `quarkctl node pause cpu`).
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
quarkctl system deploy -f monitor.quark.ts -n alice

# List systems in a namespace
quarkctl system list -n alice

# Get system details (node states, health)
quarkctl system get monitor -n alice

# Get the original .quark.ts source
quarkctl system source monitor -n alice

# Undeploy a system
quarkctl system delete monitor -n alice
```

### `quarkctl node` — manage individual nodes

```bash
# List nodes in a namespace (optionally within a specific system)
quarkctl node list -n alice
quarkctl node list -n alice -s monitor

# Get node details
quarkctl node get cpu -n alice -s monitor

# Lifecycle operations
quarkctl node pause cpu -n alice -s monitor
quarkctl node resume cpu -n alice -s monitor
quarkctl node drain cpu -n alice -s monitor
quarkctl node archive cpu -n alice -s monitor
quarkctl node recover cpu -n alice -s monitor
quarkctl node delete cpu -n alice -s monitor
```

### `quarkctl registry` — browse available node implementations

```bash
quarkctl registry list
quarkctl registry list --category source
quarkctl registry list --query timer
quarkctl registry get source/timer:v1
```

### `quarkctl event` — query the event log

```bash
quarkctl event list -n alice
quarkctl event list -n alice -s monitor
quarkctl event list -n alice --kinds NODE_STATE_CHANGED
quarkctl event count -n alice
quarkctl event watch -n alice -s monitor
```

### `quarkctl health` — platform / namespace / system / node health

```bash
quarkctl health platform
quarkctl health namespace alice
quarkctl health system monitor -n alice
quarkctl health node cpu -n alice -s monitor
```

## Conceptual alignment with the server

| Server REST endpoint | CLI command |
|---------------------|-------------|
| `POST /systems/deploy` | `quarkctl system deploy -f ... -n ...` |
| `GET /systems?namespace=` | `quarkctl system list -n ...` |
| `GET /systems/{name}?namespace=` | `quarkctl system get NAME -n ...` |
| `GET /systems/{name}/source?namespace=` | `quarkctl system source NAME -n ...` |
| `DELETE /systems/{name}?namespace=` | `quarkctl system delete NAME -n ...` |
| `GET /nodes?namespace=&system=` | `quarkctl node list -n ... [-s ...]` |
| `GET /nodes/{name}?namespace=&system=` | `quarkctl node get NAME -n ... -s ...` |
| `POST /nodes/{name}/pause` | `quarkctl node pause NAME -n ... -s ...` |
| `POST /nodes/{name}/resume` | `quarkctl node resume NAME -n ... -s ...` |
| `POST /nodes/{name}/drain` | `quarkctl node drain NAME -n ... -s ...` |
| `POST /nodes/{name}/archive` | `quarkctl node archive NAME -n ... -s ...` |
| `POST /nodes/{name}/recover` | `quarkctl node recover NAME -n ... -s ...` |
| `POST /nodes/{name}/delete` | `quarkctl node delete NAME -n ... -s ...` |
| `GET /registry` | `quarkctl registry list` |
| `GET /registry/{uri}` | `quarkctl registry get URI` |
| `GET /events?namespace=` | `quarkctl event list -n ...` |
| `GET /events/count?namespace=` | `quarkctl event count -n ...` |
| `GET /health` | `quarkctl health platform` |
| `GET /health/namespaces/{ns}` | `quarkctl health namespace NS` |
| `GET /health/systems/{name}?namespace=` | `quarkctl health system NAME -n ...` |
| `GET /health/nodes/{name}?namespace=&system=` | `quarkctl health node NAME -n ... -s ...` |
