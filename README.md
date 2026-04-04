# Quark

[![CI](https://github.com/quarkloop/quark/actions/workflows/ci.yml/badge.svg)](https://github.com/quarkloop/quark/actions/workflows/ci.yml)
[![Go 1.22+](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

> Your agents. Your machine. Fully Autonomous.

**Quark** is a local runtime for autonomous multi-agent AI spaces. A space is a self-contained directory with a `Quarkfile` and `.quark/` runtime state — copy it, share it via Git, and run it anywhere.

```
quark init my-research    # scaffold a space
quark run   my-research   # launch the agent
quark activity            # stream agent activity
```

---

## How it works

A **space** is just a directory containing a `Quarkfile` and a `.quark/` directory for runtime state. When you run a space, Quark launches the agent binary, which loads the Quarkfile, discovers installed plugins from `.quark/plugins/`, and starts the LLM loop.

The agent runs a continuous planning cycle:

```
ORIENT → PLAN → DISPATCH → MONITOR → ASSESS → (repeat)
```

It reads the goal, produces an execution plan, fans out steps to subagents, invokes tools (`bash`, `read`, `write`, `web-search`), and iterates until complete.

Everything is a **plugin** — tools, agents, skills. Plugins are installed into `.quark/plugins/{type}-{name}/` and follow a standard contract: `manifest.yaml` + `SKILL.md` + optional `bin/`.

### Sessions

A **session** is a communication channel between a user (or system) and an agent. Each session owns its own LLM context window and scoped activity stream.

| Type       | Purpose                                                          |
| ---------- | ---------------------------------------------------------------- |
| `main`     | Persistent autonomous session — one per agent, survives restarts |
| `chat`     | User-created conversation thread with independent context        |
| `subagent` | Worker session for plan step execution                           |
| `cron`     | Session for scheduled task runs                                  |

Sessions use hierarchical keys: `agent:<agentID>:<type>[:<id>]`.

### Module layout

Quark is structured as a Go workspace with nine independent modules:

| Module                   | Role                                                                                     |
| ------------------------ | ---------------------------------------------------------------------------------------- |
| `core`                   | Shared foundation: JSONL store, KB interface, CLI toolkit.                               |
| `agent`                  | Agent runtime, planning loop, activity feed, subagent dispatch, model gateway integration. |
| `agent-api`              | Shared HTTP API contracts and route helpers for agent runtimes.                          |
| `agent-client`           | Shared HTTP/SSE client for direct agent access.                                          |
| `cli`                    | `quark` CLI binary — space manager, agent client, plugin manager.                        |
| `plugins/tool-bash`      | Builtin plugin: execute shell commands.                                                  |
| `plugins/tool-read`      | Builtin plugin: read regular text files.                                                 |
| `plugins/tool-write`     | Builtin plugin: write and edit regular text files.                                       |
| `plugins/tool-web-search`| Builtin plugin: web search via Brave/SerpAPI.                                            |

**Key layering**:

```text
core -> agent
core -> cli
agent-api -> agent
agent-api -> agent-client -> cli
core -> plugins/*
```

Plugins have no compile-time dependency on quark modules — they are standalone binaries that communicate via CLI arguments or HTTP.

### Binaries

| Binary       | Module                     | Role                                                        |
| ------------ | -------------------------- | ----------------------------------------------------------- |
| `agent`      | `agent`                    | Long-lived agent runtime process — HTTP server + agent loop |
| `quark`      | `cli`                      | CLI: run/stop/doctor/init/plugin/session/config/kb/plan   |
| `bash`       | `plugins/tool-bash`        | Plugin: `run` + `serve` for shell execution                 |
| `read`       | `plugins/tool-read`        | Plugin: `run` + `serve` for reading text files              |
| `write`      | `plugins/tool-write`       | Plugin: `run` + `serve` for writing and editing text files  |
| `web-search` | `plugins/tool-web-search`  | Plugin: `run` + `serve` with Brave/SerpAPI/stub             |

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

---

## Quickstart (dry-run — no API key needed)

**1. Create a space:**

```bash
quark run . --dry-run
```

**2. Stream activity:**

```bash
quark activity
```

**3. Stop:**

```bash
quark stop <agent-url>
```

---

## Quickstart (with a real LLM)

The agent auto-detects available API keys on startup. Just export your key:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
quark run .
```

Or set the model explicitly in the Quarkfile:

```yaml
quark: "1.0"
meta:
  name: my-space
model:
  provider: anthropic
  name: claude-sonnet-4.6
plugins:
  - ref: quark/tool-bash
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

## Space layout

`quark init` scaffolds this structure:

```
my-space/
├── Quarkfile               # space definition — model, plugins, permissions
└── .quark/
    ├── sessions/           # chat threads and context state
    ├── config/             # runtime configuration (model, mode, budget)
    ├── activity/           # event log (append-only)
    ├── plans/              # execution plans (JSONL files)
    ├── kb/                 # knowledge base — JSONL files
    └── plugins/            # installed plugins
        ├── tool-bash/      #   manifest.yaml + SKILL.md + bin/
        ├── tool-read/
        └── agent-researcher/
```

---

## Quarkfile reference

Minimal `Quarkfile` (model is optional — auto-detected from env keys):

```yaml
quark: "1.0"

meta:
  name: my-space

# model:                          # optional — auto-detected from env keys
#   provider: anthropic
#   name: claude-sonnet-4.6

# routing:                        # optional — model routing and fallbacks
#   fallback:
#     - provider: openai
#       model: gpt-5.4
#   rules:
#     - match: "code_.*"
#       provider: anthropic
#       model: claude-sonnet-4.6

plugins:
  - ref: quark/tool-bash
  - ref: quark/tool-read
  - ref: quark/tool-write
  - ref: quark/tool-web-search
  # - ref: quark/agent-researcher
  #   config:
  #     max_search_depth: 5

permissions:
  filesystem:
    allowed_paths: ["."]
    read_only: ["Quarkfile"]
  network:
    allowed_hosts: ["*"]
    deny: ["169.254.0.0/16"]
  tools:
    allowed: ["*"]
    denied: []
  audit:
    log_tool_calls: true
    log_llm_responses: false
    retention_days: 30

capabilities:
  spawn_agents: true
  max_workers: 3
  create_plans: true
  approval_policy: auto         # required | auto

env:
  - ANTHROPIC_API_KEY

gateway:
  token_budget_per_hour: 100000
```

---

## CLI reference

### `quark` — agent lifecycle, data, and management

```bash
# Space Commands
quark run [dir]            Launch an agent from a space directory
  --dry-run                  Use noop gateway, no API key needed
  -d, --detach               Return immediately after agent is running
  --timeout <duration>       Wait timeout (default 30s)
quark stop <agent-url>     Request graceful agent stop
quark inspect <agent-url>  Print agent details
quark init [dir]           Scaffold a new space directory
quark doctor               Diagnose space config and plugin health
quark version              Print version

# Data Commands
quark session create       Create a new session
quark session list         List all sessions
quark session get <key>    Get a session
quark session delete <key> Delete a session
quark config list          List agent configuration
quark config get <key>     Get a config value
quark config set <k> <v>   Set a config value
quark kb get <ns/key>      Read a KB entry
quark kb set <ns/key> <v>  Write a KB entry
quark kb delete <ns/key>   Delete a KB entry
quark kb list <namespace>  List keys in a namespace
quark plan list            List all plans
quark plan get <id>        Get a plan
quark plan create          Create a plan
quark plan approve <id>    Approve a plan
quark plan reject <id>     Reject a plan
quark activity append      Append an event to the activity log
quark activity query       Query activity entries

# Management Commands
quark plugin search <q>    Search the plugin registry (github.com/quarkloop/plugins)
quark plugin install <ref> Install a plugin
  ref can be:
    tool-bash              # plugin name → cloned from registry
    github.com/user/name   # git URL shallow clone
    ./local-path           # local directory
quark plugin uninstall <n> Remove a plugin
quark plugin list          List installed plugins
quark plugin info <name>   Show plugin details
quark plugin build [dir]   Validate a plugin from source
```

### Plugin binaries — dual-mode (CLI + HTTP)

```bash
# bash
bash run --cmd "ls -la"                  One-shot execution
bash serve --addr 127.0.0.1:8091        HTTP server mode

# read
read run --path ./notes.txt
read run --path ./app.py --start-line 10 --end-line 20
read serve --addr 127.0.0.1:8093

# write
write run --path ./notes.txt --content "hello"
write run --path ./notes.txt --operation replace --find hello --replace-with world
write serve --addr 127.0.0.1:8092

# web-search
web-search run --query "..."
web-search serve --addr 127.0.0.1:8090
```

Set `BRAVE_API_KEY` or `SERPAPI_KEY` for real results; stub used otherwise.

---

## Environment variables

| Variable             | Default                 | Description                                      |
| -------------------- | ----------------------- | ------------------------------------------------ |
| `QUARK_DRY_RUN`      | —                       | Set to `1` to activate the noop gateway          |
| `ANTHROPIC_API_KEY`  | —                       | Forwarded to agents that declare it in `env`     |
| `OPENAI_API_KEY`     | —                       | Forwarded to agents that declare it in `env`     |
| `OPENROUTER_API_KEY` | —                       | Forwarded to agents that declare it in `env`     |
| `ZHIPU_API_KEY`      | —                       | Forwarded to agents that declare it in `env`     |

---

## Web UI

A Next.js frontend is included in `web/`. It proxies the agent API and provides a chat interface for interacting with running agents.

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
