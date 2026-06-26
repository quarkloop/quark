# Simple Streaming Monitor Example

This example demonstrates the core Quark principle: **the `.quark.ts` file IS the program**. The user writes only TypeScript — no Java code. The CLI sends the TypeScript to the server, which evaluates it via GraalJS and executes it on an embedded NATS JetStream backbone.

## The Program

The entire program is [`system.quark.ts`](system.quark.ts). It declares nodes and their communication patterns through NATS subjects.

## How to Run

### Prerequisites

```bash
make build    # builds the server JAR + CLI binary
```

### Quick start (automated)

```bash
make run-example                    # 15-second run (default)
make run-example EXAMPLE_DURATION=30  # 30-second run
```

### Manual workflow

**Terminal 1 — start the server:**

```bash
make server-dev          # dev mode with hot reload
```

**Terminal 2 — deploy and observe:**

```bash
make cli

# Deploy the .quark.ts file — this is ALL the user does
./quark-cli/quarkctl apply -f example/simple-streaming/system.quark.ts -n alice

# Deploy the same file in a different namespace
./quark-cli/quarkctl apply -f example/simple-streaming/system.quark.ts -n bob

# Verify deployment
./quark-cli/quarkctl get systems -n alice
./quark-cli/quarkctl get nodes -n alice -s monitor

# Watch events in real time
./quark-cli/quarkctl watch events -n alice -s monitor

# Query the streaming endpoint (Alice's data only — multi-tenant isolated)
curl -N http://localhost:8081/stream/alice/monitor/stream

# Query Bob's endpoint (completely separate data)
curl -N http://localhost:8081/stream/bob/monitor/stream

# Undeploy when done
./quark-cli/quarkctl delete system monitor -n alice
./quark-cli/quarkctl delete system monitor -n bob
```

## Multi-Tenant Isolation

Both `alice` and `bob` deploy the **same** TypeScript file. The server runs them as completely independent systems with zero data leakage:

| Concern | Alice | Bob |
|---------|-------|-----|
| NATS subjects | `alice.monitor.*` | `bob.monitor.*` |
| JetStream stream | `monitor-alice` | `monitor-bob` |
| Streaming URL | `/stream/alice/monitor/stream` | `/stream/bob/monitor/stream` |
| Event log | `systems/alice/monitor/events.jsonl` | `systems/bob/monitor/events.jsonl` |

## Output

### JSON file

The `json-writer` node writes samples to `example/simple-streaming/json/system-monitor.jsonl`. Each line is a JSON object.

### Streaming endpoint (Server-Sent Events)

```
GET /stream/<namespace>/<system>/<node>
```

This is a Server-Sent Events (SSE) stream. Use `curl -N`:

```bash
curl -N http://localhost:8081/stream/alice/monitor/stream
```

## Cleanup

```bash
make clean-state    # removes ./quark-state and JSON output
```
