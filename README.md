# quark

> Pack your agents. Ship your intelligence.

**quark** is a local runtime for autonomous multi-agent AI spaces. Define a goal, declare your agents and model, and quark handles the rest — launching isolated processes, managing the supervisor→worker execution loop, persisting context across restarts, and streaming logs in real time.

```
quark init my-research    # scaffold a project
quark lock  my-research   # pin all agent refs to exact digests
quark run   my-research   # launch the space
quark logs  <id>          # watch it work
```

---

## How it works

A **space** is a long-running supervisor process that owns an agent execution loop, a knowledge base, and a model gateway. The supervisor runs a continuous planning cycle:

```
ORIENT → PLAN → DISPATCH → MONITOR → ASSESS → (repeat)
```

It reads the goal from the KB, produces a structured execution plan, fans out each step to a short-lived **worker process**, collects results back over a Unix domain socket, and iterates until the goal is complete.

### Multi-module workspace

quark is structured as a Go workspace with eight independent modules:

```
quark/
  go.work

  store/          github.com/quarkloop/store              — JSONL collection store
  kb/             github.com/quarkloop/kb                 — knowledge base + kb CLI
  agent/          github.com/quarkloop/agent              — executor, IPC, supervisor, worker, model, context
  space/          github.com/quarkloop/space              — Quarkfile, registry, repo ops + space CLI
  api-server/     github.com/quarkloop/api-server         — HTTP API and process controller
  cli/            github.com/quarkloop/cli                — quark CLI (all commands)
  tools/bash/     github.com/quarkloop/tools/bash         — bash executor tool
  tools/web-search/ github.com/quarkloop/tools/web-search — web search tool
```

**Dependency graph** (no cycles):

```
store ← kb ← agent ← api-server ← cli
store ← agent
space ← api-server ← cli
```

### Seven binaries

| Binary       | Module       | Role |
| ------------ | ------------ | ---- |
| `agent supervisor` | `agent` | Long-lived process per space — HTTP server + IPC + agent loop |
| `agent worker`     | `agent` | Short-lived step executor — connects via IPC, runs one step, exits |
| `api-server` | `api-server` | Manages space lifecycle, port allocation, restart policy |
| `quark`      | `cli`        | CLI: run/stop/ps/logs/inspect/init/lock/validate |
| `space`      | `space`      | CLI: init/lock/validate/scaffold-registry (filesystem ops) |
| `kb`         | `kb`         | CLI: get/set/delete/list on the space knowledge base |
| `bash`       | `tools/bash` | Tool: `run` (one-shot) + `serve` (HTTP skill server) |
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

# api-server
GOWORK=off go build -mod=vendor -o bin/api-server  ./api-server/cmd/api-server

# quark CLI
GOWORK=off go build -mod=vendor -o bin/quark       ./cli/cmd/quark

# agent binary (supervisor and worker modes)
GOWORK=off go build -mod=vendor -o bin/agent       ./agent/cmd/agent

# space and kb CLI tools
GOWORK=off go build -mod=vendor -o bin/space       ./space/cmd/space
GOWORK=off go build -mod=vendor -o bin/kb          ./kb/cmd/kb

# tool binaries
GOWORK=off go build -mod=vendor -o bin/bash        ./tools/bash/cmd/bash
GOWORK=off go build -mod=vendor -o bin/web-search  ./tools/web-search/cmd/web-search
```

Add `bin/` to your `PATH`:

```bash
export PATH="$PWD/bin:$PATH"
```

The `api-server` discovers the `agent` binary automatically from the same directory as its own executable.

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

**5. Stream logs:**

```bash
quark logs space-abc123
```

**6. Stop:**

```bash
quark stop space-abc123
# ✓ Space space-abc123 stopped
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
| `openai`    | `gpt-4o`, `gpt-4o-mini`               | `OPENAI_API_KEY`     |
| `zhipu`     | `glm-4-flash`, `glm-4`               | `ZHIPU_API_KEY`      |

---

## Project layout

`quark init` scaffolds this structure:

```
my-space/
├── Quarkfile               # space definition — model, agents, skills, env, restart
├── .quark/
│   └── lock.yaml           # pinned dependency digests — commit this
├── prompts/
│   └── supervisor.txt      # supervisor system prompt
├── agents/                 # optional per-agent config overrides
├── skills/                 # optional per-skill config overrides
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

Adding worker agents and skills:

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
quark lock [dir]           Pin agent/skill refs to exact SHA-256 digests
quark validate [dir]       Validate Quarkfile and lock file

# Runtime commands (require api-server)
quark run [dir]            Launch a space — streams events (Ctrl+C to detach)
  --dry-run                  Use noop gateway, no API key needed
  -d, --detach               Return immediately after space reaches running
  --timeout <duration>       Wait timeout (default 30s)

quark stop <id>            Send SIGINT and wait for clean exit
  -f, --force                Send SIGKILL instead
quark kill <id>            Force-stop (SIGKILL)
quark logs <id>            Stream live logs from ring buffer
quark events <id>          Stream lifecycle events (SSE)
quark ps                   List running spaces (-a for all)
quark inspect <id>         Print full space record as JSON

# Space management
quark space ls             List all spaces
quark space stats <id>     Show context and KB stats for a running space
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
bash run --cmd "ls -la"              One-shot execution
bash serve --addr 127.0.0.1:8091    HTTP skill server mode
```

### `web-search` — web search tool

```bash
web-search run --query "..."                        One-shot search
web-search serve --addr 127.0.0.1:8090             HTTP skill server mode
```

Set `BRAVE_API_KEY` or `SERPAPI_KEY` for real results; stub used otherwise.

---

## Registry

Agents and skills are resolved from `~/.quark/registry/`. Built-in definitions are seeded on api-server startup.

**Built-in agents:**

| Ref                       | Role                                           |
| ------------------------- | ---------------------------------------------- |
| `quark/supervisor@latest` | Orchestrates workers; drives the planning loop |
| `quark/researcher@latest` | Gathers information using web-search           |
| `quark/writer@latest`     | Drafts and edits written content               |

**Built-in skills:**

| Ref                        | Description                          |
| -------------------------- | ------------------------------------ |
| `quark/web-search@latest`  | HTTP skill — POST query, get results |

`quark lock` resolves each `ref` to a version and SHA-256 digest. Commit `.quark/lock.yaml`.

---

## Environment variables

| Variable           | Default                 | Description                                      |
| ------------------ | ----------------------- | ------------------------------------------------ |
| `QUARK_API_SERVER` | `http://127.0.0.1:7070` | Override api-server address for all CLI commands |
| `QUARK_DRY_RUN`    | —                       | Set to `1` to activate the noop gateway          |
| `ANTHROPIC_API_KEY`| —                       | Forwarded to spaces that declare it in `env`     |
| `OPENAI_API_KEY`   | —                       | Forwarded to spaces that declare it in `env`     |
| `ZHIPU_API_KEY`    | —                       | Forwarded to spaces that declare it in `env`     |

---

## IPC protocol

Workers communicate with their supervisor over a Unix domain socket at `~/.quark/agents/<space-id>/ipc.sock` using newline-delimited JSON frames:

| Direction           | Frame    | Purpose                              |
| ------------------- | -------- | ------------------------------------ |
| supervisor → worker | `task`   | Step assignment on connect           |
| worker → supervisor | `event`  | Progress update during execution     |
| worker → supervisor | `result` | Final step outcome — worker exits    |

---

## License

MIT
