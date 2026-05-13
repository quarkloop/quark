# Quark Service Architecture

Quark services move long-lived, specialized platform capabilities behind typed
gRPC contracts. The runtime can discover them, read their `SKILL.md` guidance,
and call their RPCs through a generic service tool. The supervisor also uses
services for platform-owned state, starting with space storage.

This keeps the agent runtime focused on agent lifecycle, sessions, prompts,
plans, tools, and execution. Services own durable domain behavior such as
indexing, release automation, and space metadata.

## Topology

```text
quark CLI
  |
  | HTTP
  v
supervisor
  | \
  |  \ gRPC SpaceService
  |   v
  |  services/space
  |
  | launches runtime with QUARK_* service endpoint env vars
  v
runtime
  |
  | gRPC ServiceRegistry discovery + service RPC invocation
  v
services/indexer, services/build-release, services/space, future services
```

Every service exposes its own domain service plus
`quark.service.v1.ServiceRegistry` on the same gRPC listener. The registry
returns service descriptors, RPC method descriptors, endpoint addresses, and
embedded service skills.

## Contracts

Source protobufs live under `proto/quark/<service>/v1`.

Generated Go code lives under `pkg/serviceapi/gen` and is imported by runtime,
supervisor, service binaries, adapters, and tests. Shared gRPC helpers live in
`pkg/serviceapi/servicekit`.

Current contracts:

| Proto package | Service | Owner |
| --- | --- | --- |
| `quark.service.v1` | `ServiceRegistry` | shared service discovery |
| `quark.indexer.v1` | `IndexerService` | `services/indexer` |
| `quark.buildrelease.v1` | `BuildReleaseService` | `services/build-release` |
| `quark.space.v1` | `SpaceService` | `services/space` |

Regenerate stubs after proto changes:

```bash
make proto
```

The Buf configuration is `buf.yaml`; generation settings are in
`buf.gen.yaml`. New service packages should use stable package names,
versioned directories, and `go_package` values under
`github.com/quarkloop/pkg/serviceapi/gen/quark/<service>/v1`.

## Service Shape

A service should have:

- a protobuf contract under `proto/quark/<name>/v1`
- generated stubs under `pkg/serviceapi/gen`
- a standalone module under `services/<name>`
- a binary under `services/<name>/cmd/<name>`
- service-owned domain logic under `services/<name>/pkg` or `internal`
- a `SKILL.md` describing capabilities and request/response semantics
- a `ServiceRegistry` descriptor that lists all callable RPCs and embeds the
  service skill
- health registration for the domain gRPC service
- focused unit/integration tests

Services should not import runtime internals. Runtime and supervisor callers
interact with services through generated protobuf clients or the generic runtime
service executor.

## Runtime Discovery

The runtime discovers services from environment variables:

| Variable | Meaning |
| --- | --- |
| `QUARK_SERVICE_ADDRS` | comma, semicolon, or newline separated addresses; entries can be `name=host:port` |
| `QUARK_INDEXER_ADDR` | convenience endpoint for the indexer service |
| `QUARK_BUILD_RELEASE_ADDR` | convenience endpoint for the build-release service |
| `QUARK_SPACE_SERVICE_ADDR` | space service endpoint, passed by supervisor |
| `QUARK_DISABLE_SERVICE_DISCOVERY` | set to `true` to leave the runtime service catalog off |

At startup, `runtime/pkg/services` dials each endpoint, calls
`ServiceRegistry.ListServices`, loads descriptors and skills, and appends an
"Available gRPC Services" block to the agent system prompt. If no endpoints
are configured, runtime behavior is unchanged. Discovery failures are logged
and do not prevent the runtime from starting.

When descriptors exist, the runtime exposes a `grpc-service` tool:

```json
{
  "service": "indexer",
  "method": "GetContext",
  "request": {
    "queryVector": [1, 0, 0],
    "limit": 5,
    "depth": 2
  }
}
```

The executor resolves the descriptor, decodes the JSON request into the
protobuf message type, invokes the RPC, and returns protobuf JSON. This keeps
service-specific logic out of `runtime/pkg/agent` while giving agents one
consistent service-backed capability.

## Space Service

`services/space` owns space metadata, the authoritative Quarkfile copy,
derived storage paths, agent launch environment, and doctor diagnostics. The
supervisor keeps its HTTP API for CLI compatibility, but its `space.Store`
implementation is now a gRPC adapter over `SpaceService`.

By default, `quark-supervisor start` starts an embedded local `SpaceService`
and still talks to it through gRPC. To use an external service:

```bash
space-service --addr 127.0.0.1:7303 --root /tmp/quark-spaces
quark-supervisor start --space-service 127.0.0.1:7303
```

The supervisor passes `QUARK_SPACE_SERVICE_ADDR` into launched runtimes so the
agent can discover the space service skill and RPC surface.

## Build-Release Service

`services/build-release` owns the release pipeline. The existing
`plugins/tools/build-release` plugin remains as a compatibility adapter over
the same service-owned package, so current plugin users keep working while the
business logic is no longer plugin-bound.

Run the standalone service:

```bash
build-release-service --addr 127.0.0.1:7302 --skill-dir services/build-release
```

Configure runtime discovery with:

```bash
export QUARK_BUILD_RELEASE_ADDR=127.0.0.1:7302
```

## Indexer Service

`services/indexer` is the GraphRAG storage boundary. Agents are responsible for
reading files, extracting structure, producing embeddings, and sending
structured `IndexRequest` messages. The indexer stores chunks, entities,
relations, source metadata, and queryable context in Dgraph.

Run locally with Dgraph:

```bash
indexer-service --addr 127.0.0.1:7301 --dgraph 127.0.0.1:9080 --skill-dir services/indexer
export QUARK_INDEXER_ADDR=127.0.0.1:7301
```

The indexer intentionally does not parse PDFs or call LLMs. That work belongs
to the agent side of the boundary.

## Build And Test

Build all core binaries, tool plugins, and service binaries:

```bash
make build
```

Service binaries are emitted as:

| Binary | Source |
| --- | --- |
| `bin/indexer-service` | `services/indexer/cmd/indexer` |
| `bin/build-release-service` | `services/build-release/cmd/build-release` |
| `bin/space-service` | `services/space/cmd/space` |

Run unit tests across workspace modules:

```bash
make test
```

Run the Dgraph-backed indexer E2E tests:

```bash
go test -tags e2e -v -run '^TestIndexerServiceWithRealDgraph$' ./e2e
go test -tags e2e -v -run '^TestAgentServiceCatalogIndexesUltimateBrochurePDF$' ./e2e
```

The PDF test requires Docker/Dgraph through the E2E helper and `pdftotext` in
`PATH`. It extracts `docs/ultimate-brochure.pdf`, indexes structured content
through the runtime service executor, queries the real indexer service, asserts
chunks/citations/graph context, and logs temporary artifact paths for manual
inspection.

## Migration Notes

- Runtime service awareness now lives in `runtime/pkg/services`, not in
  plugin loading code.
- `SpaceService` owns space storage behavior; supervisor HTTP handlers keep
  the existing CLI-facing API stable.
- Build-release business logic lives in `services/build-release/pkg`; the tool
  plugin is an adapter and should stay thin.
- New services should publish a registry descriptor and skill before runtime
  integrations are added.
- If a service endpoint is absent or unavailable, only that service capability
  is omitted from the agent prompt and `grpc-service` routing.
