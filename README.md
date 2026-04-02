# Quark

[![CI](https://github.com/quarkloop/quark/actions/workflows/ci.yml/badge.svg)](https://github.com/quarkloop/quark/actions/workflows/ci.yml)
[![Go 1.22+](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

> Your agents. Your machine. Fully Autonomous.

**Quark** is a local runtime for autonomous multi-agent AI spaces. Define a goal, declare your agents and model, and Quark handles the rest — launching agent runtimes for isolated workspaces, managing the supervisor→worker execution loop, persisting context across restarts, and streaming activity and logs in real time.

```
quark init my-research    # scaffold a project
quark lock  my-research   # snapshot refs into a lock file
quark run   my-research   # launch the space
quark activity <id>       # stream agent activity
```

---

## How it works

A **space** is the persisted workspace record: directory, env, restart policy, logs, and the runtime identity for a project. When you run a space, the api-server launches one long-lived **agent runtime** for it. That agent owns the execution loop, knowledge base, and model gateway.

The agent runs a continuous planning cycle:

```
ORIENT → PLAN → DISPATCH → MONITOR → ASSESS → (repeat)
```

It reads the goal from the KB, produces a structured execution plan, fans out ready steps to worker goroutines, invokes tools like `bash`, `read`, `write`, or `web-search`, and iterates until the goal is complete.

Public HTTP APIs are split by entity:

- `/api/v1/spaces` and `/api/v1/spaces/{id}` are for space lifecycle and workspace operations.
- `/api/v1/agents/{id}` is the proxied agent API exposed through the api-server.
- `/api/v1/agent` is the direct API served by a standalone agent runtime.

### Sessions

A **session** is a communication channel between a user (or system) and an agent. Each session owns its own LLM context window and scoped activity stream.

| Type       | Purpose                                                          |
| ---------- | ---------------------------------------------------------------- |
| `main`     | Persistent autonomous session — one per agent, survives restarts |
| `chat`     | User-created conversation thread with independent context        |
| `subagent` | Worker session for plan step execution                           |
| `cron`     | Session for scheduled task runs                                  |

Sessions use hierarchical keys: `agent:<agentID>:<type>[:<id>]`. The main session is created automatically when an agent starts. Chat sessions are created via `POST /sessions` and deleted via `DELETE /sessions/{key}`. Chat messages include an optional `session_key` field to route to a specific session.

### Multi-module workspace

Quark is structured as a Go workspace with twelve independent modules:

| Module             | Role                                                                                     |
| ------------------ | ---------------------------------------------------------------------------------------- |
| `core`             | Shared foundation: JSONL store, KB abstraction, CLI toolkit.                             |
| `agent`            | Agent runtime, planning loop, activity feed, worker dispatch, model gateway integration. |
| `agent-api`        | Shared HTTP API contracts and route helpers for agent runtimes.                          |
| `agent-client`     | Shared HTTP/SSE client for direct and proxied agent access.                              |
| `api-server`       | Space lifecycle controller and HTTP API server.                                          |
| `cli`              | `quark` CLI binary.                                                                      |
| `tools/bash`       | Tool: execute shell commands.                                                            |
| `tools/kb`         | Tool: KB get/set/delete/list CLI.                                                        |
| `tools/read`       | Tool: read regular text files.                                                           |
| `tools/space`      | Quarkfile parsing, scaffold/lock/validate flows.                                         |
| `tools/write`      | Tool: write and edit regular text files.                                                 |
| `tools/web-search` | Tool: web search via Brave/SerpAPI.                                                      |

**Key layering**:

```text
core -> agent -> tools/space
agent-api -> agent-client -> api-server -> cli
agent-api -> agent
core -> tools/bash, tools/kb, tools/read, tools/write, tools/web-search
```

### Nine binaries

| Binary       | Module             | Role                                                        |
| ------------ | ------------------ | ----------------------------------------------------------- |
| `agent`      | `agent`            | Long-lived agent runtime process — HTTP server + agent loop |
| `api-server` | `api-server`       | Manages space lifecycle, port allocation, restart policy    |
| `quark`      | `cli`              | CLI: run/stop/ps/logs/activity/inspect/init/lock/validate   |
| `bash`       | `tools/bash`       | Tool: `run` + `serve` for shell execution                   |
| `kb`         | `tools/kb`         | CLI: get/set/delete/list on the knowledge base              |
| `read`       | `tools/read`       | Tool: `run` + `serve` for reading text files                |
| `space`      | `tools/space`      | CLI: init/lock/validate                                     |
| `write`      | `tools/write`      | Tool: `run` + `serve` for writing and editing text files    |
| `web-search` | `tools/web-search` | Tool: `run` + `serve` with Brave/SerpAPI/stub               |

---

## Requirements

- **Go 1.22+**
- An API key for your LLM provider — or use `--dry-run` to test without one

---

## Install

```bash
git clone https://github.com/quarkloop/quark
cd quark
make build
```

Add `bin/` to your `PATH`:

```bash
export PATH="$PWD/bin:$PATH"
```

The `api-server` discovers the `agent` binary automatically from the same directory as its own executable.

Individual builds are also available, for example `make build-agent`, `make build-tools-read`, and `make build-tools-write`.

---

## Quickstart (dry-run — no API key needed)

**1. Start the api-server** in a separate terminal:

```bash
api-server
# quark api-server v0.1.0
# api-server listening on 127.0.0.1:7070
```

**2. Create a space:**

```bash
quark init my-space
cd my-space
```

**3. Lock:**

```bash
quark lock .
# Lock file written → .quark/lock.yaml
```

**4. Run with the noop model gateway:**

```bash
quark run . --dry-run --detach
# ✓ Space created
#   ID:      space-abc123
#   Name:    my-space
#   Restart: on-failure
# ✓ Space running (detached)
#   ID:   space-abc123
#   Port: 7100
```

**5. Stream activity:**

```bash
quark activity space-abc123
```

Use `quark logs space-abc123` if you want the raw process logs instead of structured activity.

**6. Stop:**

```bash
quark stop space-abc123
# ✓ Agent space-abc123 stopped
```

---

## Quickstart (with a real LLM)

The agent auto-detects available API keys on startup. Just export your key:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
quark lock .
quark run . --detach
quark logs <id>
```

Or set the model explicitly in the Quarkfile:

```yaml
model:
  provider: anthropic
  name: claude-opus-4-6

env:
  - ANTHROPIC_API_KEY
```

Supported providers:

| Provider     | Model examples                                      | Environment variable |
| ------------ | --------------------------------------------------- | -------------------- |
| `anthropic`  | `claude-opus-4-6`, `claude-sonnet-4-6`              | `ANTHROPIC_API_KEY`  |
| `openai`     | `gpt-4o`, `gpt-4o-mini`                             | `OPENAI_API_KEY`     |
| `openrouter` | `openai/gpt-4o-mini`, `anthropic/claude-3.7-sonnet` | `OPENROUTER_API_KEY` |
| `zhipu`      | `glm-4-flash`, `glm-4`                              | `ZHIPU_API_KEY`      |

---

## Project layout

`quark init` scaffolds this structure:

```
my-space/
├── Quarkfile               # space definition — agents, tools, permissions, capabilities
├── .quark/
│   ├── lock.yaml           # lock file — commit this
│   ├── kb/                 # knowledge base — JSONL files
│   │   ├── config/         # goal, dynamic config, mode
│   │   ├── plans/          # execution plan history
│   │   ├── memory/         # agent memory entries
│   │   ├── documents/      # ingested documents
│   │   ├── notes/          # freeform notes written by agents
│   │   └── artifacts/      # step output artifacts
│   ├── skills/             # space-level skills (SKILL.md)
│   └── plugins/            # downloaded plugin content (future)
├── prompts/
│   └── supervisor.txt      # supervisor system prompt
└── agents/                 # optional per-agent prompt overrides
```

All runtime artifacts live in `.quark/`. The Quarkfile stays at the root alongside your own files.

---

## Quarkfile reference

Minimal `Quarkfile` (model is optional — auto-detected from env keys):

```yaml
quark: "1.0"

meta:
  name: my-space

# model:                          # optional — auto-detected from env keys
#   provider: anthropic
#   name: claude-opus-4-6

supervisor:
  agent: quark/supervisor
  prompt: ./prompts/supervisor.txt

permissions:
  filesystem:
    allowed_paths: ["./", "/tmp"]
    read_only: []
  network:
    allowed_hosts: ["api.openai.com", "api.anthropic.com"]
    deny: ["10.0.0.0/8"]
  tools:
    allowed: ["bash", "read", "write"]
    denied: []
  plugins:
    allowed: []
    auto_install: false
  audit:
    log_tool_calls: true
    log_llm_responses: false
    retention_days: 30

capabilities:
  spawn_agents: true
  max_workers: 10
  create_plans: true
  approval_policy: required       # required | auto

env:
  - ANTHROPIC_API_KEY

restart: on-failure
```

Adding worker agents and tools:

```yaml
agents:
  - ref: quark/researcher
    name: researcher
  - ref: quark/writer
    name: writer

tools:
  - ref: quark/bash
    name: bash
    config:
      endpoint: "http://127.0.0.1:8091/run"
  - ref: quark/web-search
    name: web-search
    config:
      endpoint: "http://127.0.0.1:8090/search"
```

**Restart policies:**

| Value        | Behaviour                           |
| ------------ | ----------------------------------- |
| `on-failure` | Restart on non-zero exit (default)  |
| `always`     | Restart on any exit including clean |
| `never`      | Do not restart                      |

Max 5 restarts with a 10-second cooldown. Restart counter survives api-server restarts.

---

## CLI reference

### `quark` — runtime and repo commands

```bash
# Project commands
quark init [dir]           Scaffold a new space project
quark lock [dir]           Snapshot refs into .quark/lock.yaml
quark validate [dir]       Validate Quarkfile and lock file

# Runtime commands (require api-server)
quark run [dir]            Launch a space — streams activity (Ctrl+C to detach)
  --dry-run                  Use noop gateway, no API key needed
  -d, --detach               Return immediately after space reaches running
  --timeout <duration>       Wait timeout (default 30s)

quark stop <id>            Request graceful agent stop and wait for exit
  -f, --force                Send SIGKILL through the space controller
quark kill <id>            Force-stop a running agent (SIGKILL)
quark logs <id>            Stream live logs from ring buffer
quark activity <id>        Stream agent activity (SSE)
quark ps                   List running spaces (-a for all)
quark inspect <id>         Print details for a space and its attached agent

# Space management
quark space ls             List all spaces
quark space stats <id>     Show runtime stats for the agent attached to a running space
quark space rm <id>        Delete a stopped/failed space record
quark space prune          Remove all stopped and failed records

# System
quark system status        Check api-server connectivity
quark version              Print version
```

### `space` — filesystem operations

```bash
space init [dir]           Scaffold a new space directory
space lock [dir]           Resolve refs and write .quark/lock.yaml
space validate [dir]       Validate Quarkfile and lock file
```

### `kb` — knowledge base

```bash
kb [--dir <dir>] get <namespace/key>         Read an entry
kb [--dir <dir>] set <namespace/key> <value> Write an entry
kb [--dir <dir>] delete <namespace/key>      Delete an entry
kb [--dir <dir>] list <namespace>            List keys in a namespace
```

### `bash` — bash executor tool

```bash
bash run --cmd "ls -la"             One-shot execution
bash serve --addr 127.0.0.1:8091   HTTP server mode
```

### `read` — text file reader

```bash
read run --path ./notes.txt
read run --path ./app.py --start-line 10 --end-line 20
read serve --addr 127.0.0.1:8093
```

### `write` — text file writer/editor

```bash
write run --path ./notes.txt --content "hello"
write run --path ./notes.txt --operation replace --find hello --replace-with world
write run --path ./app.py --operation edit --start-line 2 --start-column 1 --end-line 2 --end-column 14 --new-text "print('hi')"
write serve --addr 127.0.0.1:8092
```

### `web-search` — web search tool

```bash
web-search run --query "..."                    One-shot search
web-search serve --addr 127.0.0.1:8090         HTTP server mode
```

Set `BRAVE_API_KEY` or `SERPAPI_KEY` for real results; stub used otherwise.

---

## Environment variables

| Variable             | Default                 | Description                                      |
| -------------------- | ----------------------- | ------------------------------------------------ |
| `QUARK_API_SERVER`   | `http://127.0.0.1:7070` | Override api-server address for all CLI commands |
| `QUARK_DRY_RUN`      | —                       | Set to `1` to activate the noop gateway          |
| `ANTHROPIC_API_KEY`  | —                       | Forwarded to spaces that declare it in `env`     |
| `OPENAI_API_KEY`     | —                       | Forwarded to spaces that declare it in `env`     |
| `OPENROUTER_API_KEY` | —                       | Forwarded to spaces that declare it in `env`     |
| `ZHIPU_API_KEY`      | —                       | Forwarded to spaces that declare it in `env`     |

---

## Web UI

A Next.js frontend is included in `web/`. It proxies the agent API and provides a chat interface for interacting with running spaces.

```bash
cd web
bun install
bun dev     # starts on http://localhost:3000
```

Requires [Bun](https://bun.sh).

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, module structure, code style, and the PR process.

---

## License

Apache License 2.0
