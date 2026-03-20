# quark

> Pack your agents. Ship your intelligence.

**quark** is a local runtime for autonomous multi-agent AI spaces. Define a goal, declare your agents and model, and quark handles the rest — launching agent runtimes for isolated workspaces, managing the supervisor→worker execution loop, persisting context across restarts, and streaming activity and logs in real time.

```
quark init my-research    # scaffold a project
quark lock  my-research   # pin all agent refs to exact digests
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

### Multi-module workspace

quark is structured as a Go workspace with twelve independent modules:

| Module | Role |
| ------ | ---- |
| `core` | Shared foundation: JSONL store, KB abstraction, CLI toolkit. |
| `agent` | Agent runtime, planning loop, activity feed, worker dispatch, model gateway integration. |
| `agent-api` | Shared HTTP API contracts and route helpers for agent runtimes. |
| `agent-client` | Shared HTTP/SSE client for direct and proxied agent access. |
| `api-server` | Space lifecycle controller and HTTP API server. |
| `cli` | `quark` CLI binary. |
| `tools/bash` | Tool: execute shell commands. |
| `tools/kb` | Tool: KB get/set/delete/list CLI. |
| `tools/read` | Tool: read regular text files. |
| `tools/space` | Quarkfile parsing, registry, scaffold/lock/validate flows. |
| `tools/write` | Tool: write and edit regular text files. |
| `tools/web-search` | Tool: web search via Brave/SerpAPI. |

**Key layering**:

```text
core -> agent -> tools/space
agent-api -> agent-client -> api-server -> cli
agent-api -> agent
core -> tools/bash, tools/kb, tools/read, tools/write, tools/web-search
```

### Nine binaries

| Binary       | Module       | Role |
| ------------ | ------------ | ---- |
| `agent`      | `agent`      | Long-lived agent runtime process — HTTP server + agent loop |
| `api-server` | `api-server` | Manages space lifecycle, port allocation, restart policy |
| `quark`      | `cli`        | CLI: run/stop/ps/logs/activity/inspect/init/lock/validate |
| `bash`       | `tools/bash` | Tool: `run` + `serve` for shell execution |
| `kb`         | `tools/kb`   | CLI: get/set/delete/list on the knowledge base |
| `read`       | `tools/read` | Tool: `run` + `serve` for reading text files |
| `space`      | `tools/space` | CLI: init/lock/validate/scaffold-registry |
| `write`      | `tools/write` | Tool: `run` + `serve` for writing and editing text files |
| `web-search` | `tools/web-search` | Tool: `run` + `serve` with Brave/SerpAPI/stub |

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

**3. Lock dependencies:**

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

Edit the `Quarkfile` and set your provider and model, then export your API key:

```yaml
model:
  provider: anthropic
  name: claude-opus-4-6

env:
  - ANTHROPIC_API_KEY
```

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
quark lock .
quark run . --detach
quark logs <id>
```

Supported providers:

| Provider    | Model examples                         | Environment variable |
| ----------- | -------------------------------------- | -------------------- |
| `anthropic` | `claude-opus-4-6`, `claude-sonnet-4-6` | `ANTHROPIC_API_KEY`  |
| `openai`    | `gpt-4o`, `gpt-4o-mini`                | `OPENAI_API_KEY`     |
| `openrouter` | `openai/gpt-4o-mini`, `anthropic/claude-3.7-sonnet` | `OPENROUTER_API_KEY` |
| `zhipu`     | `glm-4-flash`, `glm-4`                 | `ZHIPU_API_KEY`      |

---

## Project layout

`quark init` scaffolds this structure:

```
my-space/
├── Quarkfile               # space definition — model, agents, tools, env, restart
├── .quark/
│   └── lock.yaml           # pinned dependency digests — commit this
├── prompts/
│   └── supervisor.txt      # supervisor system prompt
├── agents/                 # optional per-agent config overrides
├── skills/                 # optional per-tool config overrides
└── kb/                     # knowledge base — JSONL files
    ├── config/             # goal and space config
    ├── plans/              # execution plan history
    ├── memory/             # agent memory entries
    ├── documents/          # ingested documents
    ├── notes/              # freeform notes written by agents
    └── artifacts/          # step output artifacts
```

---

## Quarkfile reference

Minimal `Quarkfile`:

```yaml
quark: "1.0"

meta:
  name: my-space

model:
  provider: anthropic
  name: claude-opus-4-6

supervisor:
  agent: quark/supervisor@latest
  prompt: ./prompts/supervisor.txt

env:
  - ANTHROPIC_API_KEY

restart: on-failure
```

Adding worker agents and tools:

```yaml
agents:
  - ref: quark/researcher@latest
    name: researcher
  - ref: quark/writer@latest
    name: writer

skills:
  - ref: quark/web-search@latest
    name: web-search
    config:
      max_results: "10"
```

**Restart policies:**

| Value        | Behaviour                                         |
| ------------ | ------------------------------------------------- |
| `on-failure` | Restart on non-zero exit (default)                |
| `always`     | Restart on any exit including clean               |
| `never`      | Do not restart                                    |

Max 5 restarts with a 10-second cooldown. Restart counter survives api-server restarts.

---

## CLI reference

### `quark` — runtime and repo commands

```bash
# Project commands
quark init [dir]           Scaffold a new space project
quark lock [dir]           Pin agent/tool refs to exact SHA-256 digests
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
quark registry scaffold    Seed ~/.quark/registry/ with built-in definitions
quark version              Print version
```

### `space` — filesystem operations

```bash
space init [dir]           Scaffold a new space directory
space lock [dir]           Resolve refs and write .quark/lock.yaml
space validate [dir]       Validate Quarkfile and lock file
space scaffold-registry    Seed ~/.quark/registry/
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

## Registry

Agents and tools are resolved from `~/.quark/registry/`. Built-in definitions are seeded on api-server startup.

**Built-in agents:**

| Ref                       | Role                                           |
| ------------------------- | ---------------------------------------------- |
| `quark/supervisor@latest` | Orchestrates workers; drives the planning loop |
| `quark/researcher@latest` | Gathers information using web-search           |
| `quark/writer@latest`     | Drafts and edits written content               |

**Built-in tools:**

| Ref                       | Description |
| ------------------------- | ----------- |
| `quark/bash@latest`       | Execute a shell command |
| `quark/read@latest`       | Read a regular text file |
| `quark/write@latest`      | Write or edit a regular text file |
| `quark/web-search@latest` | Search the web through the configured provider |

`quark lock` resolves each `ref` to a version and SHA-256 digest. Commit `.quark/lock.yaml`.

---

## Environment variables

| Variable           | Default                 | Description                                      |
| ------------------ | ----------------------- | ------------------------------------------------ |
| `QUARK_API_SERVER`  | `http://127.0.0.1:7070` | Override api-server address for all CLI commands |
| `QUARK_DRY_RUN`     | —                       | Set to `1` to activate the noop gateway          |
| `ANTHROPIC_API_KEY` | —                       | Forwarded to spaces that declare it in `env`     |
| `OPENAI_API_KEY`    | —                       | Forwarded to spaces that declare it in `env`     |
| `OPENROUTER_API_KEY`| —                       | Forwarded to spaces that declare it in `env`     |
| `ZHIPU_API_KEY`     | —                       | Forwarded to spaces that declare it in `env`     |

---

## License

MIT
