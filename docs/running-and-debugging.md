# Running & Debugging

Practical techniques for running, testing, and debugging Quark locally.

## 1. Building

```bash
make build
```

Binaries land in `./bin/`:

| Binary | Role |
| --- | --- |
| `supervisor` | HTTP supervisor daemon |
| `runtime` | agent runtime process |
| `quark` | CLI |
| `bash`, `fs`, `web-search`, `build-release` | tool plugin binaries |
| `indexer-service`, `build-release-service`, `space-service` | gRPC services |

Tool plugin `.so` files and provider `.so` files are built with:

```bash
make build-plugins
```

Regenerate protobuf/gRPC stubs after proto changes:

```bash
make proto
```

## 2. Starting The Supervisor

The supervisor is the normal entrypoint for local development. It starts an
embedded `SpaceService` by default and launches runtimes on demand.

```bash
./bin/supervisor start --port 7200 --agent ./bin/runtime
```

Use an external space service when debugging service boundaries:

```bash
./bin/space-service --addr 127.0.0.1:7303 --root /tmp/quark-spaces --skill-dir services/space
./bin/supervisor start --port 7200 --agent ./bin/runtime --space-service 127.0.0.1:7303
```

The CLI finds the supervisor through `QUARK_SUPERVISOR_URL`; the default is
`http://127.0.0.1:7200`.

## 3. Starting Services

Services are gRPC processes that publish `ServiceRegistry` metadata and service
skills. The runtime discovers them through environment variables.

```bash
# Requires Dgraph Alpha on 127.0.0.1:9080
./bin/indexer-service --addr 127.0.0.1:7301 --dgraph 127.0.0.1:9080 --skill-dir services/indexer
export QUARK_INDEXER_ADDR=127.0.0.1:7301

./bin/build-release-service --addr 127.0.0.1:7302 --skill-dir services/build-release
export QUARK_BUILD_RELEASE_ADDR=127.0.0.1:7302
```

Multiple arbitrary services can also be passed with:

```bash
export QUARK_SERVICE_ADDRS='indexer=127.0.0.1:7301,build-release=127.0.0.1:7302'
```

See [services.md](services.md) for service topology, protobuf conventions,
lifecycle, and E2E instructions.

## 4. Tool Servers

Tool plugins can run in api mode as standalone HTTP servers. The runtime prefers
lib mode when a `.so` is installed next to the manifest and falls back to api
mode otherwise.

```bash
./bin/bash serve --addr 127.0.0.1:8091
./bin/fs serve --addr 127.0.0.1:8093
./bin/web-search serve --addr 127.0.0.1:8090
```

Set `BRAVE_API_KEY` or `SERPAPI_KEY` for real web search results; otherwise the
web-search plugin uses its stub behavior.

## 5. Starting A Runtime Directly

Most workflows should use `quark run`, which asks the supervisor to launch the
runtime with the correct space and service environment. Direct runtime startup
is useful for debugging the agent process itself.

```bash
export QUARK_MODEL_PROVIDER=openrouter
export QUARK_MODEL_NAME=openai/gpt-4o-mini
export QUARK_SUPERVISOR_URL=http://127.0.0.1:7200
export QUARK_SPACE=my-space
export QUARK_PLUGINS_DIR=/tmp/quark-spaces/my-space/plugins
export OPENROUTER_API_KEY=sk-or-v1-...

./bin/runtime start --port 8765 --channel web
```

The web channel exposes the runtime HTTP API on the selected port.

## 6. CLI Flow

```bash
export QUARK_SUPERVISOR_URL=http://127.0.0.1:7200

mkdir /tmp/my-space
cd /tmp/my-space
quark init --name my-space
quark run --timeout 30s
quark activity query --follow
quark stop
```

All `quark` commands operate on the space named by the local `Quarkfile`.

## 7. Testing Via curl

Health:

```bash
curl -s http://127.0.0.1:8765/api/v1/agent/health | jq .
```

Chat:

```bash
curl -s -X POST http://127.0.0.1:8765/api/v1/agent/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"what time is it?","mode":"ask","session_key":"agent:supervisor:main"}' | jq .
```

Activity:

```bash
curl -s http://127.0.0.1:8765/api/v1/agent/activity | jq .
curl -N http://127.0.0.1:8765/api/v1/agent/activity/stream
```

## 8. Running The Web UI

```bash
cd web
bun install
bun run dev
```

The dev server runs on `http://localhost:3000` and proxies API calls to the
runtime.

## 9. Tests

Run workspace unit tests:

```bash
make test
```

Run E2E tests:

```bash
go test -tags e2e -v -timeout 10m ./e2e
```

Run the service-backed indexer PDF E2E:

```bash
go test -tags e2e -v -run '^TestAgentServiceCatalogIndexesUltimateBrochurePDF$' ./e2e
```

The PDF test requires Docker/Dgraph through the E2E helper and `pdftotext` in
`PATH`. It extracts `docs/ultimate-brochure.pdf`, indexes the result through the
runtime service executor, queries the real Dgraph-backed indexer, and logs
artifact paths for manual verification.

## 10. Debugging Tips

- Use `pgrep -af 'bin/runtime|bin/supervisor|indexer-service|space-service'`
  to find running Quark processes.
- Port conflicts usually show up as `bind: address already in use`; stop the
  old process or choose another port.
- Runtime service discovery logs one line per discovered service. If a service
  is missing from the agent prompt, check the relevant `QUARK_*_ADDR`
  environment variable and that `ServiceRegistry.ListServices` is reachable.
- Provider errors such as 405, 429, or "Function calling is not enabled" are
  upstream model/provider issues. Try a model that supports tool calling or
  wait for rate limits to clear.
- If old session or context data causes confusion, stop the runtime and inspect
  the space's `sessions/` and KB directories under the configured space root.
