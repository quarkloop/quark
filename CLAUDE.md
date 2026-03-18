# quark

Local runtime for autonomous multi-agent AI spaces.

## Architecture

Go 1.22 workspace with 8 modules and 7 binaries. Each module is a standalone Go module with its own `go.mod`.

### Module Layout

| Module | Role |
|--------|------|
| `core` | Shared foundation: JSONL store, KB abstraction, CLI toolkit. No quarkloop deps. |
| `agent` | Multi-agent execution engine: supervisor loop, worker dispatch, context management. |
| `api-server` | HTTP API server for space management and agent interaction. |
| `cli` | `quark` CLI binary — user-facing entrypoint. |
| `tools/bash` | Tool: shell command execution (CLI + HTTP server). |
| `tools/kb` | Tool: knowledge base get/set/delete/list (CLI). |
| `tools/space` | Tool: space init/lock/validate/stats, Quarkfile parsing, registry. |
| `tools/web-search` | Tool: web search via Brave/SerpAPI (CLI + HTTP server). |

### Dependency Graph

```
core (leaf — no quarkloop deps)
  ↑
  ├── agent
  │     ↑
  │     ├── api-server (also depends on core, tools/space)
  │     │     ↑
  │     │     └── cli
  │     └── cli (also depends on api-server, tools/space, core)
  ├── tools/space (also depends on agent)
  ├── tools/kb
  ├── tools/bash
  └── tools/web-search
```

## Build

```bash
make build       # 7 binaries in ./bin/
make test        # tests across all modules
make vet         # go vet across all modules
make fmt         # gofmt across all modules
make tidy        # go mod tidy across all modules
make clean       # rm -rf bin/
```

Individual: `make build-agent`, `make build-tools-space`, etc.

## Development

- **Workspace mode** (default): `go.work` at root with `use` and `replace` directives. All modules resolve locally.
- **Standalone build**: `GOWORK=off go build -mod=vendor ./path/to/cmd` (requires vendored deps).
- Each module's `go.mod` has `replace` directives for standalone builds outside the workspace.

## Conventions

- CLI tools use `core/pkg/toolkit` for bootstrap (`NewToolCommand` + `Execute`).
- Cobra for all CLI commands.
- JSONL-backed key-value store (`core/pkg/store`) for persistence.
- KB abstraction (`core/pkg/kb`) wraps the store with namespace/key semantics.
- Agent plan types live in `agent/pkg/plan`.
- Tool binaries follow the pattern: `tools/<name>/cmd/<name>/main.go` (thin CLI) + `tools/<name>/pkg/<name>/` (library).
- Module paths: `github.com/quarkloop/<module>` for top-level, `github.com/quarkloop/tools/<name>` for tools.
