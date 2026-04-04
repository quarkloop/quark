# Quark

Local runtime for autonomous multi-agent AI spaces.

## Architecture

Go 1.22 workspace with 9 modules and 6 binaries. Each module is a standalone Go module with its own `go.mod`.

### Module Layout

| Module | Role |
|--------|------|
| `core` | Shared foundation: JSONL store, KB interface, CLI toolkit, space directory API. No quarkloop deps. |
| `agent` | Multi-agent execution engine: supervisor loop, subagent dispatch, context management. |
| `agent-api` | Shared HTTP API contracts and route helpers for agent access. |
| `agent-client` | Shared HTTP/SSE client for talking to agents. |
| `cli` | `quark` CLI binary — user-facing entrypoint. Directly launches and connects to agent. |
| `plugins/tool-bash` | Builtin plugin: shell command execution (CLI + HTTP server). |
| `plugins/tool-read` | Builtin plugin: read regular text files (CLI + HTTP server). |
| `plugins/tool-write` | Builtin plugin: write and edit regular text files (CLI + HTTP server). |
| `plugins/tool-web-search` | Builtin plugin: web search via Brave/SerpAPI (CLI + HTTP server). |

### Dependency Graph

```
core
  ↑
  ├── agent
  ├── agent-api
  └── cli

agent-api
  ↑
  ├── agent
  ├── agent-client
  │     ↑
  │     └── cli
  └── cli

cli
  ↑
  └── agent  (agent imports cli/pkg/kb for KB interface)

plugins/*  (no compile-time deps on quark modules)
  ↑
  └── core
```

Builtin plugins live in `plugins/` as self-contained modules. They have no compile-time dependency on quark modules — the plugin contract (manifest.yaml + SKILL.md + CLI/HTTP interface) is the only coupling.

## CLI Package Structure

The `cli` module contains all CLI logic:

| Package | Role |
|---------|------|
| `cli/cmd/quark` | Entry point binary. |
| `cli/pkg/root.go` | Root command definition with command groups. |
| `cli/pkg/commands/` | All command implementations (init, run, stop, doctor, plugin, session, config, kb, plan, activity, version). |
| `cli/pkg/commands/plugin/` | Plugin install, uninstall, list, info, build, search commands. |
| `cli/pkg/config/` | Build-time version info. |
| `cli/pkg/kb/` | KB Store interface + JSONL-backed local implementation. |
| `cli/pkg/kbclient/` | Unified KB client (local filesystem or HTTP transport). |
| `cli/pkg/kbserver/` | HTTP server handlers for the KB REST API. |
| `cli/pkg/middleware/` | PersistentPreRunE hooks (RequireSpace). |
| `cli/pkg/plugin/` | Plugin manifest parsing, discovery, remote ops (clone, copy). |
| `cli/pkg/quarkfile/` | Quarkfile struct, parsing, validation. |
| `cli/pkg/space/` | Space init and validate operations. |

## Build

```bash
make build       # 6 binaries in ./bin/
make test        # tests across all modules
make test-e2e    # E2E tests (requires OPENROUTER_API_KEY or ZHIPU_API_KEY)
make vet         # go vet across all modules
make lint        # staticcheck across all modules
make fmt         # gofmt across all modules
make tidy        # go mod tidy across all modules
make clean       # rm -rf bin/
```

Individual: `make build-agent`, `make build-cli`, `make build-tool-bash`, `make build-tool-read`, `make build-tool-write`, `make build-tool-web-search`.

## Development

- **Workspace mode** (default): `go.work` at root with `use` and `replace` directives. All modules resolve locally.
- **Standalone build**: `GOWORK=off go build -mod=vendor ./path/to/cmd` (requires vendored deps).
- Each module's `go.mod` has `replace` directives for standalone builds outside the workspace.

## Agent Package Structure

The `agent` module is split into focused sub-packages with strict single responsibility:

| Package | Role |
|---------|------|
| `agent/pkg/agentcore` | Shared types, constants, `Resources` struct. Leaf dependency — no logic. |
| `agent/pkg/session` | Session types, hierarchical keys, KB-backed CRUD store. |
| `agent/pkg/inference` | LLM calling: `Infer`, `InferWithRetry`, message factory helpers. |
| `agent/pkg/execution` | Tool invocation and plan step execution (LLM+tool loop). |
| `agent/pkg/chat` | Chat mode routing: ask, plan, masterplan, auto. Prompt builders. |
| `agent/pkg/cycle` | Supervisor loop: orient → plan → dispatch → monitor → assess. |
| `agent/pkg/agent` | Thin orchestrator: session management, request routing, lifecycle glue. |
| `agent/pkg/config` | KB-backed dynamic config store with owner-wins semantics. |
| `agent/pkg/eventbus` | In-memory pub/sub with per-subscriber channels and typed event kinds. |
| `agent/pkg/hooks` | Extensible interception: Observer/Modifier/Gate hooks at tool and inference points. |
| `agent/pkg/intervention` | Per-session message queue for mid-execution user course-correction. |
| `agent/pkg/model` | LLM provider abstraction: Anthropic, OpenAI, OpenRouter, Zhipu, noop. |
| `agent/pkg/context` | LLM context management: token accounting, compaction, visibility policies. |
| `agent/pkg/activity` | Persisted event log — async subscriber to EventBus with ring buffer. |
| `agent/pkg/tool` | HTTP tool dispatcher and tool definition types. |
| `agent/pkg/plan` | Plan and step types, KB-backed stores, master plan support. |
| `agent/pkg/runtime` | Agent lifecycle: process management, HTTP server. |
| `agent/pkg/plugin` | Plugin manifest parsing, discovery from `.quark/plugins/`, hub client. |

**Dependency graph** (no circular imports):
```
agentcore (types, constants, resources)
   ↑
   ├── inference
   ├── execution (→ inference, hooks)
   ├── chat (→ inference, execution, intervention)
   ├── cycle (→ inference, intervention)
   ├── session (→ cli/pkg/kb only)
   ├── config (→ cli/pkg/kb)
   ├── eventbus (leaf)
   ├── hooks (→ eventbus)
   ├── intervention (leaf)
   └── activity (→ eventbus, cli/pkg/kb)

agent (→ all of the above)
```

## Session Model

Sessions are communication channels (chat threads), not process lifecycle records. Each session owns its own `AgentContext` (LLM message window) and scoped activity stream.

### Session Types

| Type | Purpose |
|------|---------|
| `main` | Persistent autonomous session. Created once per agent, survives restarts. Cannot be deleted. |
| `chat` | User-created conversation thread. Independent context per chat. |
| `subagent` | Worker session for plan step execution. Created/destroyed by supervisor. |
| `cron` | Session for scheduled task runs. |

### Session Keys

Hierarchical key scheme: `agent:<agentID>:<type>[:<id>]`

- Main: `agent:supervisor:main`
- Chat: `agent:supervisor:chat:a1b2c3d4`
- SubAgent: `agent:supervisor:subagent:step-1`
- Cron: `agent:supervisor:cron:daily:run:5`

### Session API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/sessions` | GET | List all sessions for the agent |
| `/sessions` | POST | Create a new session (`{"type":"chat","title":"..."}`) |
| `/sessions/{sessionKey}` | GET | Get a specific session |
| `/sessions/{sessionKey}` | DELETE | Delete a session (cannot delete main) |
| `/sessions/{sessionKey}/activity` | GET | Get session-scoped activity |
| `/chat` | POST | Chat in a session (`session_key` field routes to session) |

## Agent Working Modes

The agent supports four dynamic working modes, set per-session via `ChatRequest.Mode`:

| Mode | Behaviour |
|------|-----------|
| `ask` | Direct answer with optional tool use. No plans created. |
| `plan` | Creates a single execution plan with steps. Run() loop dispatches approved plans. |
| `masterplan` | Creates a multi-phase master plan. Each phase becomes its own sub-plan. |
| `auto` | LLM classifies the request and routes to ask/plan/masterplan. |

Mode is per-session. Default is `auto`. Main session mode persists in KB across restarts.

### Approval Policy

`ApprovalPolicy` on `agentcore.Config` controls plan approval:

| Policy | Constant | Behaviour |
|--------|----------|-----------|
| `required` | `ApprovalRequired` | Plans are created as drafts; user must approve before execution. |
| `auto` | `ApprovalAuto` | Plans are auto-approved for immediate execution. |

## Plugins

Everything is a plugin. Three types exist:

| Type | What it contains | Examples |
|------|-----------------|----------|
| **tool** | Binary (CLI + HTTP) + manifest.yaml + SKILL.md | `tool-bash`, `tool-read`, `tool-write`, `tool-web-search` |
| **agent** | System prompt + skill references + tool requirements | `agent-supervisor`, `agent-researcher` |
| **skill** | Guidance files only, no binary | `skill-code-review`, `skill-debugging` |

Installed plugins live in `.quark/plugins/{type}-{name}/`:
```
.quark/plugins/
├── tool-bash/
│   ├── manifest.yaml
│   ├── SKILL.md
│   └── bin/bash
├── tool-read/
│   ├── manifest.yaml
│   └── bin/read
└── agent-researcher/
    ├── manifest.yaml
    └── SKILL.md
```

## E2E Tests

E2E tests live in `agent/e2e/` and use the `//go:build e2e` build tag. They cover full agent flows against real providers, including binary-backed tests that start the real `agent`, `bash`, `read`, and `write` processes and drive them through the shared HTTP client.

```bash
# From the workspace root:
go test -tags e2e -v -timeout 10m ./agent/e2e

# Override provider/model via env:
OPENROUTER_API_KEY=sk-... OPENROUTER_E2E_MODEL=... go test -tags e2e -v -timeout 10m ./agent/e2e
ZHIPU_API_KEY=... ZHIPU_E2E_MODEL=... go test -tags e2e -v -timeout 10m ./agent/e2e
```

Provider resolution order: `OPENROUTER_API_KEY` first, then `ZHIPU_API_KEY`. The e2e helpers load `quark/.env` before checking the process environment.

## Conventions

- CLI tools use `core/pkg/toolkit` for bootstrap (`NewToolCommand` + `Execute`).
- Cobra for all CLI commands.
- JSONL-backed key-value store (`core/pkg/store`) for persistence.
- KB abstraction (`cli/pkg/kb`) wraps the store with namespace/key semantics.
- Agent plan types live in `agent/pkg/plan`.
- Shared agent types (Definition, Mode, ApprovalPolicy, ChatRequest/Response) live in `agent/pkg/agentcore`.
- Session types and store live in `agent/pkg/session`.
- Tool dispatch types live in `agent/pkg/tool`.
- Shared agent HTTP contracts live in `agent-api`; reusable HTTP/SSE access lives in `agent-client`.
- Plugin binaries follow the pattern: `plugins/{type}-{name}/cmd/{name}/main.go` (thin CLI) + `plugins/{type}-{name}/pkg/{name}/` (library).
- Module paths: `github.com/quarkloop/<module>` for top-level, `github.com/quarkloop/plugins/{type}-{name}` for plugins.
