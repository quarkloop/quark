# API reference

Complete reference for the Quark platform's external APIs: the REST API exposed by the control plane, and the `quarkctl` CLI commands.

For architecture, see [ARCHITECTURE.md](./ARCHITECTURE.md). For NATS wire protocol, see [PROTOCOL.md](./PROTOCOL.md).

## Table of contents

- [REST API](#rest-api)
  - [Namespaces](#namespaces)
  - [Systems](#systems)
  - [Nodes](#nodes)
  - [Events](#events)
  - [Registry](#registry)
  - [Health](#health)
- [CLI (`quarkctl`)](#cli-quarkctl)
  - [Apply (declarative)](#apply-declarative)
  - [Get (queries)](#get-queries)
  - [Watch (streams)](#watch-streams)
  - [Delete](#delete)
  - [Node registry](#node-registry)

---

## REST API

The control plane (`quark-server`) exposes a REST API on port 8080 (override with `QUARK_HTTP_PORT`). All responses are JSON.

### Namespaces

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/namespaces` | List all active namespaces |
| GET | `/api/v1/namespaces/{ns}` | Get namespace details + metrics |

### Systems

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/namespaces/{ns}/systems` | List systems in a namespace |
| GET | `/api/v1/namespaces/{ns}/systems/{name}` | Get system details |
| PUT | `/api/v1/namespaces/{ns}/systems/{name}` | Apply (declarative reconcile) |
| DELETE | `/api/v1/namespaces/{ns}/systems/{name}` | Undeploy a system |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/source` | Get the original `.quark.ts` source |

### Nodes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/namespaces/{ns}/systems/{name}/nodes` | List nodes in a system |
| GET | `/api/v1/namespaces/{ns}/systems/{name}/nodes/{node}` | Get node details |

### Events

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/namespaces/{ns}/events` | Query events |

### Registry

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/registry` | List registered node implementations |

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/q/health/live` | Liveness check (SmallRye Health default path) |
| GET | `/q/health/ready` | Readiness check (NATS, Catalog, registry) |

---

## CLI (`quarkctl`)

The `quarkctl` binary in `quark-cli/` is the operator's primary interface. It talks to the control plane's REST API over HTTP.

### Apply (declarative)

Deploy a `.quark.ts` file under a namespace:

```bash
quarkctl apply -f monitor.quark.ts -n alice
```

This is the equivalent of `PUT /api/v1/namespaces/alice/systems/monitor` with the file contents as the body. The control plane persists the source, forwards it to the data plane via NATS, and the data plane evaluates it with GraalJS.

### Get (queries)

```bash
# List systems / nodes / namespaces
quarkctl get systems -n alice
quarkctl get nodes -n alice -s monitor
quarkctl get namespaces

# Get system / node details
quarkctl get system monitor -n alice
quarkctl get node cpu -n alice -s monitor

# Query events
quarkctl get events -n alice
```

### Watch (streams)

```bash
# Stream events as they happen
quarkctl watch events -n alice
```

### Delete

```bash
# Delete a system (undeploy)
quarkctl delete system monitor -n alice
```

### Node registry

```bash
# List / search / inspect registered nodes
quarkctl node list
quarkctl node info quark/time/schedule/timer:v1
quarkctl node search timer

# Push a node package to the Catalog
quarkctl node push -f my-node.ts --uri acme/data/payments/risk-score:v1

# Pull a node package from the Catalog (debugging)
quarkctl node pull acme/data/payments/risk-score:v1
```

### JSON output (for AI agents and scripting)

Most `get` commands support a `--json` flag that emits structured JSON instead of human-readable tables:

```bash
quarkctl get system monitor -n alice --json
quarkctl get nodes -n alice -s monitor --json
quarkctl get namespaces --json
```

This is the recommended mode for scripting, CI/CD, and AI-agent integrations.

---

## SDK clients

For programmatic access from TypeScript applications, use the [`@quarkloop/quark-js`](https://github.com/quarkloop/quark-js) SDK. It provides a typed client over the same REST API plus direct NATS access for execute / batch / pipeline operations.
