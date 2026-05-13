# Quark

<!-- [![CI](https://github.com/quarkloop/quark/actions/workflows/ci.yml/badge.svg)](https://github.com/quarkloop/quark/actions/workflows/ci.yml) -->
[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

> Autonomous AI Agents for Everyday Work - Your Agents. Your Machine. Your Control.

**Quark** is an autonomous multi-agent runtime. Your working directory contains a single `Quarkfile`; the supervisor stores the latest space state under its own space directory.

```
quark-supervisor start    # long-running daemon (once per machine)
quark init my-research    # scaffold a Quarkfile and register the space
quark run                 # launch the agent for this space
quark activity query -f   # stream agent activity
```

---

## How it works

Quark has three core processes plus optional gRPC services:

- **`quark-supervisor`** - a long-running local daemon. Owns every space's latest Quarkfile state, KB, installed plugins, sessions, and the registry of running agents. Speaks HTTP.
- **`agent`** - a child process launched by the supervisor for a specific space. Runs the LLM loop, handles chat and plans. Speaks HTTP.
- **`quark`** - a thin CLI. Talks only to the supervisor (and, for chat/session/plan/activity, to the supervisor-resolved agent URL). The CLI reads and writes exactly one local file: the `Quarkfile` in the current directory.
- **Services** - independently deployable gRPC capabilities such as `indexer`, `build-release`, and `space`. Services publish descriptors and `SKILL.md` content through `quark.service.v1.ServiceRegistry`.

A **space** is identified by its `meta.name` in the Quarkfile. The supervisor keeps the authoritative copy under `$QUARK_SPACES_ROOT` (default `~/.quarkloop/spaces/<name>/`); the Quarkfile you edit in your working directory is re-submitted to the supervisor when you update it.

The agent runs a continuous planning cycle:

```
ORIENT → PLAN → DISPATCH → MONITOR → ASSESS → (repeat)
```

It reads the goal, produces an execution plan, fans out steps to subagents, invokes tools (`bash`, `fs`, `web-search`), and iterates until complete.

Everything user-installable is a **plugin** - tools, providers, agents, skills. Tool plugins support both **lib mode** (loaded in-process as Go `.so` files) and **api mode** (run as separate HTTP server processes); the agent prefers lib mode and falls back to api when the `.so` is absent. Provider plugins are always lib mode. Services are not plugins: they are platform processes with protobuf contracts, service registry metadata, and service-owned skills.

### Sessions

A **session** is a communication channel between a user (or system) and an agent. Each session owns its own LLM context window and scoped activity stream.

| Type       | Purpose                                                          |
| ---------- | ---------------------------------------------------------------- |
| `main`     | Persistent autonomous session - one per agent, survives restarts |
| `chat`     | User-created conversation thread with independent context        |
| `subagent` | Worker session for plan step execution                           |
| `cron`     | Session for scheduled task runs                                  |

Sessions use hierarchical keys: `agent:<agentID>:<type>[:<id>]`.

### Module layout

Quark is a Go workspace. See [AGENTS.md](AGENTS.md) for the full package-level breakdown.

| Module                             | Role                                                                          |
| ---------------------------------- | ----------------------------------------------------------------------------- |
| `supervisor`                       | Long-running daemon: space store, agent registry, plugin manager, HTTP API + Go SDK. |
| `runtime`                          | Agent runtime, planning loop, activity feed, subagent dispatch.                |
| `cli`                              | `quark` CLI - HTTP-only client, reads and writes only the local Quarkfile.     |
| `pkg/space`                        | Shared space directory model and Quarkfile schema, validation, and I/O.        |
| `pkg/plugin`                       | Shared plugin interfaces, manifest parsing, lib/api loader.                 |
| `pkg/serviceapi`                   | Shared protobuf/gRPC stubs and service helper utilities.                      |
| `pkg/toolkit`                      | Shared toolkit for tool plugins (Fiber server, CLI, pipe mode).               |
| `services/indexer`                 | Dgraph-backed GraphRAG indexing and retrieval service.                        |
| `services/build-release`           | Standalone build and release automation service.                              |
| `services/space`                   | Space metadata, Quarkfile, path, environment, and doctor service.             |
| `plugins/tools/{bash,fs,web-search,build-release}` | Tool plugins (lib-mode `.so` + api-mode HTTP daemon).             |
| `plugins/providers/{openrouter,openai,anthropic}` | Provider plugins (lib mode `.so`).                              |

**Key layering:**

```text
pkg/plugin ← supervisor/pkg/pluginmanager, plugins/tools/*, plugins/providers/*
supervisor ← cli (via supervisor/pkg/client)
agent      ← cli (via agent/pkg/client, for session/plan/activity/chat)
```

The CLI has no dependency on the `agent` module's internals - only on the public `agent/pkg/client` SDK. Plugins have no compile-time dependency on agent or cli.

### Binaries

| Binary            | Module                              | Role                                                                |
| ----------------- | ----------------------------------- | ------------------------------------------------------------------- |
| `quark-supervisor`| `supervisor`                        | Long-running daemon managing all spaces and agent lifecycle         |
| `runtime`         | `runtime`                           | Agent runtime process - launched on demand by the supervisor        |
| `quark`           | `cli`                               | CLI: init / run / stop / inspect / doctor / plugin / session / config / kb / plan / activity |
| `bash`            | `plugins/tools/bash`                | Tool plugin: shell execution                                        |
| `fs`              | `plugins/tools/fs`                  | Tool plugin: filesystem read/write operations                       |
| `web-search`      | `plugins/tools/web-search`          | Tool plugin: web search via Brave / SerpAPI / stub                  |
| `build-release`   | `plugins/tools/build-release`       | Tool plugin: build and release automation                           |
| `indexer-service` | `services/indexer`                  | gRPC GraphRAG indexing service backed by Dgraph                     |
| `build-release-service` | `services/build-release`      | gRPC build and release automation service                           |
| `space-service`   | `services/space`                    | gRPC space metadata and Quarkfile service                           |

---

## Requirements

- **Go 1.26+**
- An API key for your LLM provider - or use `noop` provider settings to test without one

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

## Quickstart

**1. Start the supervisor** (once per machine - leave it running in a terminal or under systemd/launchd):

```bash
quark-supervisor start
```

Override storage location with `QUARK_SPACES_ROOT`, or the listen port with `--port`. The CLI finds the supervisor at `QUARK_SUPERVISOR_URL` (default `http://127.0.0.1:7200`). The supervisor starts an embedded `SpaceService` by default; pass `--space-service <host:port>` to use an external one.

**2. Scaffold and register a space:**

```bash
mkdir my-research && cd my-research
quark init --name my-research
```

`init` writes a `Quarkfile` locally and registers the space with the supervisor.

**3. Launch the agent:**

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
quark run
```

**4. Stream activity and stop:**

```bash
quark activity query --follow
quark stop
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

Your working directory contains **only** the Quarkfile:

```
my-space/
└── Quarkfile               # space definition - model, plugins, permissions
```

Everything else lives under the supervisor's storage root (`$QUARK_SPACES_ROOT`, default `~/.quarkloop/spaces/`):

```
~/.quarkloop/spaces/my-space/
├── meta.json               # space metadata (name, version, timestamps)
├── Quarkfile               # latest Quarkfile state
├── kb/                     # knowledge base
├── plugins/                # installed plugins
│   ├── tool-bash/          #   manifest.yaml + SKILL.md + bin/
│   ├── tool-fs/
│   └── agent-researcher/
└── sessions/               # one JSONL file per session
```

This layout is an implementation detail of the supervisor's `FSStore` and is not part of the stable contract - always operate on a space through the CLI or supervisor API.

---

## Quarkfile reference

Minimal `Quarkfile`:

```yaml
quark: "1.0"

meta:
  name: my-space
  version: "0.1.0"

model:
  provider: anthropic
  name: claude-sonnet-4
  env:
    - ANTHROPIC_API_KEY

# routing:                        # optional - model routing and fallbacks
#   fallback:
#     - provider: openai
#       model: gpt-5.4
#   rules:
#     - match: "code_.*"
#       provider: anthropic
#       model: claude-sonnet-4.6

plugins:
  - ref: quark/tool-bash
  - ref: quark/tool-fs
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

gateway:
  token_budget_per_hour: 100000
```

---

## CLI reference

All `quark` commands operate on the space defined by the Quarkfile in the current working directory. The supervisor must be running.

### `quark-supervisor` — daemon

```bash
quark-supervisor start              Start the HTTP daemon
  --port <n>                          Listen port (default 7200)
  --agent <path>                      Path to agent binary (default "agent")
  --space-service <addr>              External SpaceService gRPC address
  [spaces-dir]                        Override QUARK_SPACES_ROOT
quark-supervisor stop               Stop a running supervisor
```

### `quark` — space lifecycle, data, and management

```bash
# Space lifecycle
quark init [dir]                    Scaffold a Quarkfile and register the space
  --name <name>                       Space name (defaults to directory name)
quark run [dir]                     Start the agent for the current space
  --port <n>                          Agent HTTP port (0 = supervisor picks)
  --timeout <duration>                Wait for agent to become ready (default 30s)
quark stop                          Stop the running agent
quark inspect                       Show space metadata and agent status
quark doctor                        Validate Quarkfile and plugin health
quark version                       Print version

# Data (per-space, via supervisor)
quark config list                   List configuration keys
quark config get <key>              Read a config value
quark config set <key> <value>      Write a config value
quark config delete <key>           Delete a config value
quark kb list <namespace>           List keys in a KB namespace
quark kb get <ns/key>               Read a KB entry
quark kb set <ns/key> <val|@file>   Write a KB entry
quark kb delete <ns/key>            Delete a KB entry

# Agent (requires a running agent)
quark session create                Create a new session
  --type <chat|subagent|cron>         Session type (default chat)
  --title <title>                     Session title
quark session list                  List all sessions
quark session get <key>             Get a session
quark session delete <key>          Delete a session (main cannot be deleted)
quark plan get                      Fetch the agent's current plan
quark plan list                     List the current plan summary
quark plan approve [plan-id]        Approve a plan
quark plan reject  [plan-id]        Reject a plan
quark activity query                Query the activity log
  --type <event-type>                 Filter by event type
  --limit <n>                         Max entries (default 50)
  -f, --follow                        Stream live events

# Plugins (per-space, via supervisor)
quark plugin list                   List installed plugins
  --type <tool|provider|agent|skill>  Filter by plugin type
quark plugin info <name>            Show installed plugin details
quark plugin install <ref>          Install a plugin into the space
  ref can be:
    bash                             # registry name
    github.com/user/tool-foo         # git URL
    ./local-plugin                   # local directory
quark plugin uninstall <name>       Remove a plugin
quark plugin search <query>         Search the plugin hub
```

### Plugin binaries — dual-mode (CLI + HTTP)

```bash
# bash
bash run --cmd "ls -la"                  One-shot execution
bash serve --addr 127.0.0.1:8091        HTTP server mode

# fs
fs run --path ./notes.txt
fs run --path ./app.py --start-line 10 --end-line 20
fs run --path ./notes.txt --operation write --content "hello"
fs serve --addr 127.0.0.1:8093

# web-search
web-search run --query "..."
web-search serve --addr 127.0.0.1:8090
```

Set `BRAVE_API_KEY` or `SERPAPI_KEY` for real results; stub used otherwise.

---

## Services

Service architecture, runtime discovery, protobuf conventions, lifecycle, and
E2E instructions are documented in [docs/services.md](docs/services.md).

Quick local examples:

```bash
# Dgraph must already be listening on 127.0.0.1:9080
indexer-service --addr 127.0.0.1:7301 --dgraph 127.0.0.1:9080 --skill-dir services/indexer
export QUARK_INDEXER_ADDR=127.0.0.1:7301

build-release-service --addr 127.0.0.1:7302 --skill-dir services/build-release
export QUARK_BUILD_RELEASE_ADDR=127.0.0.1:7302

space-service --addr 127.0.0.1:7303 --root /tmp/quark-spaces --skill-dir services/space
quark-supervisor start --space-service 127.0.0.1:7303
```

---

## Environment variables

| Variable               | Default                       | Description                                             |
| ---------------------- | ----------------------------- | ------------------------------------------------------- |
| `QUARK_SUPERVISOR_URL` | `http://127.0.0.1:7200`       | Supervisor base URL used by the CLI and agent           |
| `QUARK_SPACES_ROOT`    | `~/.quarkloop/spaces`         | Supervisor's on-disk root for space storage             |
| `QUARK_SERVICE_ADDRS`  | —                             | Additional runtime service endpoints (`name=host:port`) |
| `QUARK_INDEXER_ADDR`   | —                             | Runtime discovery endpoint for `IndexerService`         |
| `QUARK_BUILD_RELEASE_ADDR` | —                         | Runtime discovery endpoint for `BuildReleaseService`    |
| `QUARK_SPACE_SERVICE_ADDR` | —                         | Runtime discovery endpoint for `SpaceService`           |
| `QUARK_DISABLE_SERVICE_DISCOVERY` | —                    | Disable runtime service discovery when set to `true`    |
| `ANTHROPIC_API_KEY`    | —                             | Forwarded to agents that declare it in `env`            |
| `OPENAI_API_KEY`       | —                             | Forwarded to agents that declare it in `env`            |
| `OPENROUTER_API_KEY`   | —                             | Forwarded to agents that declare it in `env`            |
| `ZHIPU_API_KEY`        | —                             | Forwarded to agents that declare it in `env`            |

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
