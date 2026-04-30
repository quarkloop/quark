# Running & Debugging

Practical techniques for running, testing, and debugging the Quark agent runtime locally.

---

## 1. Building

```bash
# Build all 9 binaries into ./bin/
make build
```

Binaries land in `./bin/`: `supervisor`, `agent`, `quark`, `bash`, `fs`, `web-search`.

---

## 2. Starting Tool Servers

Each tool runs as a standalone HTTP server. Start them before the agent:

```bash
./bin/bash serve --addr 127.0.0.1:8091 &
./bin/read serve --addr 127.0.0.1:8092 &
./bin/write serve --addr 127.0.0.1:8093 &
```

The `--addr` flag sets the listen address (not `--port`). Defaults:

| Tool  | Default Address |
| ----- | --------------- |
| bash  | 127.0.0.1:8091  |
| read  | 127.0.0.1:8092  |
| write | 127.0.0.1:8093  |

Check tool help with `./bin/bash serve --help`.

Logs go to stdout. Redirect to files for debugging:

```bash
./bin/bash serve --addr 127.0.0.1:8091 > /tmp/bash.log 2>&1 &
```

### Common Issue: Address Already in Use

If a tool fails with `bind: address already in use`, find and kill the existing process:

```bash
pgrep -f 'bin/bash' | xargs kill
# or
lsof -i :8091
```

---

## 3. Starting the Agent

The agent needs an API key exported in the environment. It reads tools from the Quarkfile.

```bash
export OPENROUTER_API_KEY=sk-or-v1-...
./bin/agent run --dir /tmp/test-space --port 7100 --id supervisor
```

| Flag           | Description                                | Default |
| -------------- | ------------------------------------------ | ------- |
| `--dir`        | Space directory containing the Quarkfile   | `.`     |
| `--port`       | HTTP API port                              | 7100    |
| `--id`         | Agent ID (required)                        | —       |
| `--api-server` | Optional api-server URL for health reports | —       |

### Environment Variables

The agent reads env vars listed in the Quarkfile's `env:` section. For OpenRouter:

```bash
export OPENROUTER_API_KEY=sk-or-v1-...
```

The `.env` file at the workspace root has keys for E2E tests but is **not** auto-loaded by the agent binary. You must `source .env` or `export` the key manually.

### Background with Logs

```bash
export OPENROUTER_API_KEY=sk-or-v1-...
: > /tmp/agent.log
./bin/agent run --dir /tmp/test-space --port 7100 --id supervisor >> /tmp/agent.log 2>&1 &
tail -f /tmp/agent.log
```

---

## 4. Initializing a Test Space

```bash
rm -rf /tmp/test-space
./bin/space init /tmp/test-space
```

This creates a Quarkfile and prompt directory. Edit `/tmp/test-space/Quarkfile` to set the model and provider.

---

## 5. Changing the Model

Edit the `model:` section in the Quarkfile:

```yaml
model:
  provider: openrouter
  name: stepfun/step-3.5-flash:free
```

The agent must be restarted after changing the model — it reads the Quarkfile at startup.

### Free Models on OpenRouter

Not all free models support function/tool calling. If you see errors like `Function calling is not enabled`, switch to a model that supports it. StepFun's free models generally support tool calling via text parsing (fenced blocks).

---

## 6. Testing via curl

### Chat (ask mode)

```bash
curl -s -X POST http://127.0.0.1:7100/api/v1/agent/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"what time is it?","mode":"ask","session_key":"agent:supervisor:main"}' | jq .
```

### List Sessions

```bash
curl -s http://127.0.0.1:7100/api/v1/agent/sessions | jq .
```

### Create a Chat Session

```bash
curl -s -X POST http://127.0.0.1:7100/api/v1/agent/sessions \
  -H 'Content-Type: application/json' \
  -d '{"type":"chat","title":"debug session"}' | jq .
```

### Get Activity (all events)

```bash
curl -s http://127.0.0.1:7100/api/v1/agent/activity | jq .
```

### Get Session-Scoped Activity

```bash
curl -s http://127.0.0.1:7100/api/v1/agent/sessions/agent:supervisor:main/activity | jq .
```

### Filter Activity by Type

```bash
# Tool events only
curl -s http://127.0.0.1:7100/api/v1/agent/activity | \
  jq '.[] | select(.type == "tool.called" or .type == "tool.completed")'

# Check that tool events have session_id
curl -s http://127.0.0.1:7100/api/v1/agent/activity | \
  jq '.[] | select(.type == "tool.called") | {id, type, session_id, tool: .data.tool}'
```

### Health Check

```bash
curl -s http://127.0.0.1:7100/api/v1/agent/health | jq .
```

---

## 7. Running the Web UI

```bash
cd web
bun install
bun run dev
```

The dev server runs on `http://localhost:3000`. It proxies API calls to the agent via Next.js API routes.

Build for production:

```bash
cd web && bun run build
```

---

## 8. Full Local Stack

Complete startup sequence:

```bash
# 1. Build everything
make build

# 2. Start tool servers
./bin/bash serve --addr 127.0.0.1:8091 > /tmp/bash.log 2>&1 &
./bin/read serve --addr 127.0.0.1:8092 > /tmp/read.log 2>&1 &
./bin/write serve --addr 127.0.0.1:8093 > /tmp/write.log 2>&1 &

# 3. Init a space (skip if already exists)
./bin/space init /tmp/test-space

# 4. Start agent
export OPENROUTER_API_KEY=sk-or-v1-...
./bin/agent run --dir /tmp/test-space --port 7100 --id supervisor > /tmp/agent.log 2>&1 &

# 5. Start web UI
cd web && bun run dev &

# 6. Open browser
open http://localhost:3000
```

### Teardown

```bash
pkill -f 'bin/agent'
pkill -f 'bin/bash.*serve'
pkill -f 'bin/read.*serve'
pkill -f 'bin/write.*serve'
pkill -f 'next dev'
```

---

## 9. Debugging Techniques

### Agent Not Responding

1. Check the agent log: `tail -f /tmp/agent.log`
2. Verify tool servers are running: `pgrep -a 'bin/bash|bin/read|bin/write'`
3. Test the agent directly: `curl http://127.0.0.1:7100/api/v1/agent/health`

### Tool Calls Not Appearing in UI

The UI filters activity events by `session_id`. If tool events don't have a `session_id`, they are invisible.

Verify events have `session_id`:

```bash
curl -s http://127.0.0.1:7100/api/v1/agent/activity | \
  jq '.[] | select(.type == "tool.called") | {id, session_id}'
```

If `session_id` is empty/null, the issue is in `emitActivity` — it must receive and set the session key.

### Model Returning JSON Instead of Plain Text

Some models (especially cheaper/free ones) wrap plain text answers in JSON like `{"message":"Hello!"}`. The `unwrapJSONMessage` helper in `chat/mode_ask.go` handles this. If a model keeps doing it:

1. Check the system prompt — it should say "Do NOT wrap your response in JSON"
2. Check `FencedBlockParser.FormatHint()` — it instructs models on output format
3. The `unwrapJSONMessage` function recursively extracts string values from JSON objects

### Model Using Wrong Fence Labels

Models may use ` ```bash ` or ` ```json ` instead of ` ```tool ` for tool calls. The `FencedBlockParser` in `model/parser_fenced.go` accepts multiple fence labels: `tool`, `skill`, `json`, `bash`.

### Provider Errors (405, 429, etc.)

These are upstream provider issues, not bugs in Quark:

- **405**: Provider WAF blocking requests (seen with StepFun/Alibaba). Usually transient — retry later or switch models.
- **429**: Rate limited. The free tier on OpenRouter has strict limits.
- **400 "Function calling is not enabled"**: The model doesn't support native tool calling. Quark falls back to text-parsed tool calls via fenced blocks, but the gateway may still try native calls first.

### Context Snapshot Issues

The agent saves context snapshots to KB. If restoring from a stale snapshot causes issues:

```bash
# Check stored snapshots
ls /tmp/test-space/.quark/store/

# Nuclear option: wipe state and restart fresh
rm -rf /tmp/test-space/.quark/
./bin/agent run --dir /tmp/test-space --port 7100 --id supervisor
```

### SSE Stream Debugging

The web UI uses Server-Sent Events for real-time activity. To debug the stream directly:

```bash
curl -N http://127.0.0.1:7100/api/v1/agent/activity/stream
```

This shows raw SSE events as they arrive. Each event is a JSON-encoded `ActivityRecord`.

---

## 10. E2E Tests

```bash
# Run all E2E tests (requires API key)
OPENROUTER_API_KEY=sk-or-v1-... go test -tags e2e -v -timeout 10m ./agent/e2e

# The e2e helpers auto-load .env from the workspace root
go test -tags e2e -v -timeout 10m ./agent/e2e
```

E2E tests start real binary processes (agent, bash, read, write) and drive them through the HTTP client.

---

## 11. Process Management Tips

- Use `pgrep -f 'bin/agent'` to find running agent processes (not just `pgrep agent` which matches too broadly).
- Always kill old processes before starting new ones — port conflicts give `bind: address already in use`.
- The `exit code 144` from bash backgrounding is normal (SIGTERM + 128) — it means the previous process was killed.
- Clear log files before restarting: `: > /tmp/agent.log` (truncates without removing).
