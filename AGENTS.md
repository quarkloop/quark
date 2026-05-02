# Quark

Local runtime for autonomous multi-agent AI spaces.

## Architecture

Go 1.26 workspace with 14 modules and 7 binaries. Each module is a standalone Go module with its own `go.mod`.

### Module Layout

| Module                         | Role                                                                                    |
| ------------------------------ | --------------------------------------------------------------------------------------- |
| `runtime`                      | Multi-agent runtime: supervisor loop, subagent dispatch, context management.            |
| `cli`                          | `quark` CLI binary — user-facing entrypoint. Directly launches and connects to runtime. |
| `supervisor`                   | Supervisor process for managing runtime lifecycle.                                      |
| `pkg/plugin`                   | Shared plugin interfaces, types, manifest parsing, and loader for lib/api modes.        |
| `pkg/space`                    | Shared space directory model + Quarkfile schema/validation                             |
| `pkg/toolkit`                  | Shared toolkit for tool plugins (Fiber server, CLI, pipe mode).                        |
| `plugins/tools/bash`           | Tool plugin: shell command execution (lib + api).                                       |
| `plugins/tools/fs`             | Tool plugin: filesystem read/write operations (lib + api).                             |
| `plugins/tools/web-search`     | Tool plugin: web search via Brave/SerpAPI (lib + api).                                  |
| `plugins/tools/build-release`   | Tool plugin: build and release automation (lib + api).                                 |
| `plugins/providers/openrouter` | Provider plugin: OpenRouter API (lib mode .so).                                         |
| `plugins/providers/openai`     | Provider plugin: OpenAI API (lib mode .so).                                             |
| `plugins/providers/anthropic`  | Provider plugin: Anthropic Messages API (lib mode .so).                                 |
| `e2e`                          | End-to-end tests with real providers (build tag: e2e).                                |

### Dependency Graph

```
pkg/plugin (shared plugin interfaces and types)
  ↑
  ├── supervisor/pkg/pluginmanager  (plugin install/lookup per space)
  ├── plugins/tools/*                (implement plugin.ToolPlugin, lib + api modes)
  └── plugins/providers/*            (implement plugin.ProviderPlugin, lib mode)

pkg/space (shared space directory model + Quarkfile schema/validation)
  ↑
  ├── supervisor/pkg/space            (FSStore adapter and supervisor-owned operations)
  └── cli/pkg/commands/*              (local Quarkfile reads for current-space resolution)

supervisor (owns all persistent state)
  ├── supervisor/pkg/space      (FSStore: metadata, latest Quarkfile, KB/plugins/sessions dirs)
  ├── supervisor/pkg/kb         (per-space KB store)
  ├── supervisor/pkg/registry   (in-memory agent process registry)
  ├── supervisor/pkg/runtime    (launches agent processes)
  ├── supervisor/pkg/server     (HTTP API)
  └── supervisor/pkg/client     (Go SDK for the supervisor HTTP API)

cli (HTTP-only; reads/writes exactly one local file: the Quarkfile in cwd)
  ├── supervisor/pkg/client     (for all supervisor operations)
  └── runtime/pkg/client        (for direct agent calls: session/plan/activity/chat)

runtime (launched by supervisor; speaks HTTP)
  └── pkg/plugin                (manifest + loader types)
```

**Process model.** The supervisor is a long-running HTTP daemon that owns all persistent state. The CLI is a thin HTTP client — it never touches the filesystem except to read/write the `Quarkfile` in the current working directory. Agents are child processes launched by the supervisor on demand.

Tool plugins support both modes: **lib mode** (Go `.so` loaded in-process via `plugin.Open()`) and **api mode** (separate HTTP server process). The agent's pluginmanager prefers lib mode when the `.so` is shipped alongside the manifest and falls back to api mode otherwise. Provider plugins are always lib mode.

## CLI Package Structure

The `cli` module is an HTTP client. It reads and writes exactly one file on disk
— the `Quarkfile` in the current working directory — and delegates everything
else to the supervisor or the agent.

| Package                    | Role                                                                                                                                        |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `cli/cmd/quark`            | Entry point binary.                                                                                                                         |
| `cli/pkg/root.go`          | Root command definition with command groups.                                                                                                |
| `cli/pkg/commands/`        | Command implementations (init, run, stop, inspect, doctor, plugin, session, config, kb, plan, activity, version).                           |
| `cli/pkg/commands/plugin/` | Plugin install/uninstall/list/info/search (all via supervisor).                                                                             |
| `pkg/space`                | Shared space directory model and Quarkfile I/O/schema/validation.                                                                           |
| `cli/pkg/agentdial/`       | Resolves the running agent for the current space (supervisor → agent URL → `runtime/pkg/client`). Used by session, plan, activity commands. |
| `cli/pkg/buildinfo/`       | Build-time version info.                                                                                                                    |
| `cli/pkg/util/`            | Shared CLI helpers (formatted output).                                                                                                      |

## Supervisor Package Structure

| Package                        | Role                                                                                                                                                                         |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `supervisor/cmd/supervisor`    | Entry point binary.                                                                                                                                                          |
| `supervisor/pkg/api`           | Wire types and `RouteBuilder` (shared between server and client).                                                                                                            |
| `supervisor/pkg/client`        | Go SDK for the supervisor HTTP API — split by concern: `client.go` (transport + `HTTPError`, `IsNotFound`, `IsConflict`), `spaces.go`, `kb.go`, `plugins.go`, `runtimes.go`. |
| `supervisor/pkg/commands`      | `supervisor start` / `supervisor stop` cobra commands.                                                                                                                       |
| `supervisor/pkg/server`        | HTTP handlers (`handler.go`, `space_handler.go`, `kb_handler.go`, `plugin_handler.go`, `agent_handler.go`, `session_handler.go`) + routes — Fiber v2.                        |
| `supervisor/pkg/space`         | `Store` interface, `FSStore` implementation, `Doctor` function.                                                                                                              |
| `supervisor/pkg/kb`            | Per-space JSONL-backed KB. Opened via `store.KB(name)`.                                                                                                                      |
| `supervisor/pkg/pluginmanager` | Per-space plugin install/list/uninstall. Opened via `store.Plugins(name)`.                                                                                                   |
| `supervisor/pkg/runtime`       | In-memory registry of running `Runtime` processes; launches and reaps runtime child processes.                                                                               |

### Space storage layout (FSStore)

Rooted at `$QUARK_SPACES_ROOT` or `$HOME/.quarkloop/spaces`:

```
<root>/<name>/
├── meta.json                   # {name, version, created_at, updated_at}
├── Quarkfile                   # latest Quarkfile state only
├── kb/                         # per-space knowledge base
├── plugins/                    # installed plugins for this space
└── sessions/                   # one JSONL file per session
```

Spaces are keyed by `meta.name` from the Quarkfile — never by path.

## Build

```bash
make build           # Build runtime, cli, supervisor, and tool plugin binaries
make build-plugins   # Build all plugins (tool binaries + tool .so + provider .so)
make build-tools     # Build tool plugins as binaries to bin/
make build-tools-lib # Build tool plugins as .so files in-tree (requires CGO)
make build-providers # Build provider plugins as .so files in-tree (requires CGO)
make test            # Tests across all modules
make test-e2e        # E2E tests (requires OPENROUTER_API_KEY or ZHIPU_API_KEY)
make vet             # go vet across all modules
make lint            # staticcheck across all modules
make fmt             # gofmt across all modules
make tidy            # go mod tidy across all modules
make clean           # rm -rf bin/ and plugin .so files
```

Individual: `make build-runtime`, `make build-cli`, `make build-supervisor`.

## Development

- **Workspace mode** (default): `go.work` at root with `use` and `replace` directives. All modules resolve locally.
- **Standalone build**: `GOWORK=off go build -mod=vendor ./path/to/cmd` (requires vendored deps).
- Each module's `go.mod` has `replace` directives for standalone builds outside the workspace.
- **Go toolchain**: The `go.mod` files specify `go 1.26`. Go 1.21+ auto-downloads the required toolchain when the system Go is older (see `go env GOTOOLCHAIN`). To avoid auto-downloads, install Go 1.26.2 system-wide at `/mnt/devtools/tools/go`.

## Runtime Package Structure

The `runtime` module is split into focused sub-packages with strict single responsibility:

| Package                    | Role                                                                                |
| -------------------------- | ----------------------------------------------------------------------------------- |
| `runtime/pkg/agentcore`    | Shared types, constants, `Resources` struct. Leaf dependency — no logic.            |
| `runtime/pkg/session`      | Session types, hierarchical keys, KB-backed CRUD store.                             |
| `runtime/pkg/inference`    | LLM calling: `Infer`, `InferWithRetry`, message factory helpers.                    |
| `runtime/pkg/execution`    | Tool invocation and plan step execution (LLM+tool loop).                            |
| `runtime/pkg/chat`         | Chat mode routing: ask, plan, masterplan, auto. Prompt builders.                    |
| `runtime/pkg/cycle`        | Supervisor loop: orient → plan → dispatch → monitor → assess.                       |
| `runtime/pkg/api`          | Consolidated HTTP handlers (system, agent, message, channel) — Fiber v2.            |
| `runtime/pkg/agent`        | Thin orchestrator: session management, request routing, lifecycle glue.             |
| `runtime/pkg/config`       | KB-backed dynamic config store with owner-wins semantics.                           |
| `runtime/pkg/eventbus`     | In-memory pub/sub with per-subscriber channels and typed event kinds.               |
| `runtime/pkg/hooks`        | Extensible interception: Observer/Modifier/Gate hooks at tool and inference points. |
| `runtime/pkg/intervention` | Per-session message queue for mid-execution user course-correction.                 |
| `runtime/pkg/model`        | LLM provider abstraction: Anthropic, OpenAI, OpenRouter, Zhipu, noop.               |
| `runtime/pkg/context`      | LLM context management: token accounting, compaction, visibility policies.          |
| `runtime/pkg/activity`     | Persisted event log — async subscriber to EventBus with ring buffer.                |
| `runtime/pkg/tool`         | HTTP tool dispatcher and tool definition types.                                     |
| `runtime/pkg/plan`         | Plan and step types, KB-backed stores, master plan support.                         |
| `runtime/pkg/runtime`      | Agent lifecycle: process management, HTTP server (Fiber v2).                        |
| `runtime/pkg/plugin`       | Plugin manifest parsing, discovery from `.quark/plugins/`, hub client.              |

**Dependency graph** (no circular imports):

```
agentcore (types, constants, resources)
   ↑
   ├── inference
   ├── execution  (→ inference, hooks)
   ├── chat       (→ inference, execution, intervention)
   ├── cycle      (→ inference, intervention)
   ├── session    (KB-backed)
   ├── config     (KB-backed)
   ├── eventbus   (leaf)
   ├── hooks      (→ eventbus)
   ├── intervention (leaf)
   └── activity   (→ eventbus, KB-backed)

agent (→ all of the above)
```

The runtime is launched by the supervisor and is passed its space name and the
supervisor URL via environment (`QUARK_RUNTIME_ID`, `QUARK_SPACE`). KB and config
reads/writes go through the supervisor, not the local filesystem.

## Session Model

Sessions are communication channels (chat threads), not process lifecycle records. Each session owns its own `AgentContext` (LLM message window) and scoped activity stream.

### Session Types

| Type       | Purpose                                                                                      |
| ---------- | -------------------------------------------------------------------------------------------- |
| `main`     | Persistent autonomous session. Created once per agent, survives restarts. Cannot be deleted. |
| `chat`     | User-created conversation thread. Independent context per chat.                              |
| `subagent` | Worker session for plan step execution. Created/destroyed by supervisor.                     |
| `cron`     | Session for scheduled task runs.                                                             |

### Session Keys

Hierarchical key scheme: `agent:<agentID>:<type>[:<id>]`

- Main: `agent:supervisor:main`
- Chat: `agent:supervisor:chat:a1b2c3d4`
- SubAgent: `agent:supervisor:subagent:step-1`
- Cron: `agent:supervisor:cron:daily:run:5`

### Session API

| Endpoint                          | Method | Description                                               |
| --------------------------------- | ------ | --------------------------------------------------------- |
| `/sessions`                       | GET    | List all sessions for the agent                           |
| `/sessions`                       | POST   | Create a new session (`{"type":"chat","title":"..."}`)    |
| `/sessions/{sessionKey}`          | GET    | Get a specific session                                    |
| `/sessions/{sessionKey}`          | DELETE | Delete a session (cannot delete main)                     |
| `/sessions/{sessionKey}/activity` | GET    | Get session-scoped activity                               |
| `/chat`                           | POST   | Chat in a session (`session_key` field routes to session) |

## Agent Working Modes

The agent supports four dynamic working modes, set per-session via `ChatRequest.Mode`:

| Mode         | Behaviour                                                                         |
| ------------ | --------------------------------------------------------------------------------- |
| `ask`        | Direct answer with optional tool use. No plans created.                           |
| `plan`       | Creates a single execution plan with steps. Run() loop dispatches approved plans. |
| `masterplan` | Creates a multi-phase master plan. Each phase becomes its own sub-plan.           |
| `auto`       | LLM classifies the request and routes to ask/plan/masterplan.                     |

Mode is per-session. Default is `auto`. Main session mode persists in KB across restarts.

### Approval Policy

`ApprovalPolicy` on `agentcore.Config` controls plan approval:

| Policy     | Constant           | Behaviour                                                        |
| ---------- | ------------------ | ---------------------------------------------------------------- |
| `required` | `ApprovalRequired` | Plans are created as drafts; user must approve before execution. |
| `auto`     | `ApprovalAuto`     | Plans are auto-approved for immediate execution.                 |

## Plugins

Everything is a plugin. Four types exist:

| Type         | Mode      | What it contains                                                        | Examples                              |
| ------------ | --------- | ----------------------------------------------------------------------- | ------------------------------------- |
| **tool**     | lib + api | `plugin.so` (lib) and/or executable (binary) + manifest.yaml + SKILL.md | `bash`, `fs`, `web-search`, `build-release` |
| **provider** | lib       | .so plugin + manifest.yaml + SKILL.md                                   | `openrouter`, `openai`, `anthropic`   |
| **agent**    | -         | System prompt + skill references + tool requirements                    | `supervisor`, `researcher`            |
| **skill**    | -         | Guidance files only, no binary                                          | `code-review`, `debugging`            |

### Plugin Modes

- **Lib mode**: Plugin is a Go `.so` file loaded in-process via `plugin.Open()`. Requires CGO to build. Fastest dispatch (no HTTP). Used by all provider plugins and preferred for tool plugins.
- **API mode**: Plugin runs as a separate Fiber v2 HTTP server process. Tool plugins fall back to api mode when `plugin.so` is not shipped next to the manifest; the tool binary exposes `POST /<toolName>` for dispatch (via `pkg/toolkit/server.go`).

Tools ship both artifacts by default: `make build-tools` produces the binary under `bin/`, and `make build-tools-lib` produces `plugin.so` in each tool's source directory. Installers lay both out next to `manifest.yaml` inside the space; the agent's pluginmanager tries lib mode first and falls back to api on load failure.

### Source Directory Structure

```
plugins/
├── tools/
│   ├── bash/
│   │   ├── manifest.yaml
│   │   ├── SKILL.md
│   │   ├── plugin.go          # lib-mode export (build tag: plugin)
│   │   ├── cmd/bash/          # api-mode entry point (Fiber v2)
│   │   └── pkg/bash/          # shared implementation + RunHandler() (Fiber)
│   ├── fs/
│   ├── web-search/
│   │   ├── manifest.yaml
│   │   ├── SKILL.md
│   │   ├── cmd/web-search/   # api-mode entry point (Fiber v2)
│   │   └── pkg/websearch/    # shared implementation + searchHandler() (Fiber)
│   └── build-release/
└── providers/
    ├── openrouter/
    │   ├── manifest.yaml
    │   ├── SKILL.md
    │   ├── plugin.go          # lib-mode export
    │   └── provider.go        # ProviderPlugin implementation
    ├── openai/
    └── anthropic/
```

Agent HTTP handlers are consolidated in `runtime/pkg/api/`:

```
runtime/pkg/api/
├── system.go     # Health, Stop handlers
├── agent.go      # Info handler
├── message.go    # List, Send, Stream, Edit handlers (SSE)
└── channel.go    # ListChannels handler
```

Tool plugin api-mode servers use `pkg/toolkit/server.go` (Fiber v2):

```
pkg/toolkit/
├── server.go     # BuildServer(), RunServer() — Fiber v2 HTTP server
├── cli.go        # BuildCLI() — Cobra CLI with serve subcommand
└── pipe.go       # RunPipe() — JSON pipe mode
```

### Manifest Schema

```yaml
name: bash
version: "1.0.0"
type: tool # tool | provider | agent | skill
mode: lib # lib | api
description: "Execute shell commands"

# Type-specific config (nested)
tool:
  schema:
    name: bash
    description: "Execute a shell command"
    parameters:
      type: object
      properties:
        cmd: { type: string }
      required: [cmd]
  permissions:
    filesystem: ["*"]
    network: false

# For providers:
provider:
  api_base: "https://api.openai.com/v1"
  auth_env: "OPENAI_API_KEY"
  supports_streaming: true
```

## E2E Tests

E2E tests live in `e2e/` and use the `//go:build e2e` build tag. They cover full agent flows against real providers, starting the real `supervisor`, `agent`, and tool processes, and exercise both plugin modes: tools ship with `plugin.so` by default (lib mode), and `TestBashToolAPIMode` strips the `.so` to force api-mode fallback.

```bash
# From the workspace root:
go test -tags e2e -v -timeout 10m ./e2e

# Or via make:
make test-e2e

# Override provider/model via env:
OPENROUTER_API_KEY=sk-... OPENROUTER_E2E_MODEL=... make test-e2e
ZHIPU_API_KEY=... ZHIPU_E2E_MODEL=... make test-e2e
```

Provider resolution order: `OPENROUTER_API_KEY` first, then `ZHIPU_API_KEY`. The e2e helpers load `quark/.env` before checking the process environment.

## Environment variables

| Variable               | Consumer     | Purpose                                                                  |
| ---------------------- | ------------ | ------------------------------------------------------------------------ |
| `QUARK_SUPERVISOR_URL` | cli, runtime | Supervisor base URL (default `http://127.0.0.1:7200`).                   |
| `QUARK_SPACES_ROOT`    | supervisor   | Filesystem root for the space store (default `$HOME/.quarkloop/spaces`). |
| `QUARK_RUNTIME_ID`     | runtime      | Set by the supervisor when launching a runtime process.                  |
| `QUARK_SPACE`          | runtime      | Set by the supervisor; the space name this runtime serves.               |

## Conventions

- Cobra for all CLI commands.
- JSONL-backed key-value store for persistence.
- The CLI is HTTP-only: shared `pkg/space` helpers are the only code it uses for local Quarkfile reads.
- Agents and supervisor never look up spaces by path — spaces are keyed by `meta.name`.
- Agent plan types live in `runtime/pkg/plan`.
- Shared agent types (Definition, Mode, ApprovalPolicy, ChatRequest/Response) live in `runtime/pkg/agentcore`.
- Session types and store live in `runtime/pkg/session`.
- Tool dispatch types live in `runtime/pkg/tool`.
- Shared plugin interfaces live in `pkg/plugin` (PluginType, ToolPlugin, ProviderPlugin).
- Tool plugin binaries follow the pattern: `plugins/tools/{name}/cmd/{name}/main.go` (thin CLI) + `plugins/tools/{name}/pkg/{name}/` (library).
- Provider plugins follow the pattern: `plugins/providers/{name}/plugin.go` + `plugins/providers/{name}/provider.go`.
- Module paths: `github.com/quarkloop/<module>` for top-level, `github.com/quarkloop/plugins/tools/{name}` for tool plugins, `github.com/quarkloop/plugins/providers/{name}` for provider plugins.

## Git commit

Always commit changes considering following strict requirements:

- commit changes per scope.
- do not commit all changes into a single commit.
- do not include Co-author in commit message.
- follow the commit message convention: {type}: {description}
