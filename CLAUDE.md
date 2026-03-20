# quark

Local runtime for autonomous multi-agent AI spaces.

## Architecture

Go 1.22 workspace with 12 modules and 9 binaries. Each module is a standalone Go module with its own `go.mod`.

### Module Layout

| Module | Role |
|--------|------|
| `core` | Shared foundation: JSONL store, KB abstraction, CLI toolkit. No quarkloop deps. |
| `agent` | Multi-agent execution engine: supervisor loop, worker dispatch, context management. |
| `agent-api` | Shared HTTP API contracts and route helpers for direct and proxied agent access. |
| `agent-client` | Shared HTTP/SSE client for talking to direct agents or proxied agents. |
| `api-server` | HTTP API server for space management and agent interaction. |
| `cli` | `quark` CLI binary — user-facing entrypoint. |
| `tools/bash` | Tool: shell command execution (CLI + HTTP server). |
| `tools/kb` | Tool: knowledge base get/set/delete/list (CLI). |
| `tools/read` | Tool: read regular text files (CLI + HTTP server). |
| `tools/space` | Tool: space init/lock/validate/stats, Quarkfile parsing, registry. |
| `tools/write` | Tool: write and edit regular text files (CLI + HTTP server). |
| `tools/web-search` | Tool: web search via Brave/SerpAPI (CLI + HTTP server). |

### Dependency Graph

```
core
  ↑
  ├── agent
  │   └── tools/space
  ├── tools/bash
  ├── tools/kb
  ├── tools/read
  ├── tools/write
  └── tools/web-search

agent-api
  ↑
  ├── agent
  ├── agent-client
  └── api-server
       ↑
       └── cli
```

Public HTTP APIs are split by entity:
- `/api/v1/spaces/{id}` is only for space lifecycle and workspace operations.
- `/api/v1/agents/{id}` is only for agent operations exposed through the api-server.
- `/api/v1/agent` is the direct API served by a standalone agent runtime.

## Build

```bash
make build       # 9 binaries in ./bin/
make test        # tests across all modules
make test-e2e    # E2E tests (requires OPENROUTER_API_KEY or ZHIPU_API_KEY)
make vet         # go vet across all modules
make fmt         # gofmt across all modules
make tidy        # go mod tidy across all modules
make clean       # rm -rf bin/
```

Individual: `make build-agent`, `make build-tools-read`, `make build-tools-write`, etc.

## Development

- **Workspace mode** (default): `go.work` at root with `use` and `replace` directives. All modules resolve locally.
- **Standalone build**: `GOWORK=off go build -mod=vendor ./path/to/cmd` (requires vendored deps).
- Each module's `go.mod` has `replace` directives for standalone builds outside the workspace.

## Agent Working Modes

The agent supports four dynamic working modes, set per-message via `ChatRequest.Mode`:

| Mode | Behaviour |
|------|-----------|
| `ask` | Read-only: single LLM call, no plans, no tools. |
| `plan` | Creates a single execution plan with steps. Run() loop dispatches approved plans. |
| `masterplan` | Creates a multi-phase master plan. Each phase becomes its own sub-plan. |
| `auto` | LLM classifies the request and routes to ask/plan/masterplan. |

Mode is agent-internal (no CLI flags or api-server changes). Default is `auto`. Mode persists in KB across sessions.

### Approval Policy

`ApprovalPolicy` on `agent.Config` controls plan approval:

| Policy | Constant | Behaviour |
|--------|----------|-----------|
| `required` | `ApprovalRequired` | Plans are created as drafts; user must approve before execution. |
| `auto` | `ApprovalAuto` | Plans are auto-approved for immediate execution. |

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
- KB abstraction (`core/pkg/kb`) wraps the store with namespace/key semantics.
- Agent plan types live in `agent/pkg/plan`.
- Agent mode and approval policy types live in `agent/pkg/agent`.
- Shared agent HTTP contracts live in `agent-api`; reusable HTTP/SSE access lives in `agent-client`.
- Tool binaries follow the pattern: `tools/<name>/cmd/<name>/main.go` (thin CLI) + `tools/<name>/pkg/<name>/` (library).
- Module paths: `github.com/quarkloop/<module>` for top-level, `github.com/quarkloop/tools/<name>` for tools.
