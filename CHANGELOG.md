# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Apache License 2.0 — the project is now formally licensed under Apache 2.0 (see [LICENSE](./LICENSE)).
- [CONTRIBUTING.md](./CONTRIBUTING.md) — development setup, PR workflow, code style rules per language, and test expectations.
- [SECURITY.md](./SECURITY.md) — vulnerability reporting process, disclosure timeline, scope, and production hardening recommendations.
- [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) — Contributor Covenant v2.0.

### Changed

- Top-level directory layout renamed for clarity. The four component directories are now prefixed with `quark-`:
  - `cli/` → `quark-cli/`
  - `runtime/` → `quark-runtime/`
  - `nodes/` → `quark-nodes/`
  - `server/` → `quark-server/`
  - `quark-catalog/` was already prefixed and is unchanged.
- All path references in `Makefile`, `pom.xml`, `Dockerfile`, `scripts/*.sh`, `README.md`, and `AGENTS.md` updated to use the new directory names.
- `Dockerfile` rewritten to match the current v6 architecture. The previous version referenced module paths (`core/`, `server/quark-app/`, `runtime/providers/`) that haven't existed since the v6 refactor — it would have failed at `COPY` time.

## [0.1.0] — Pre-release

The platform is in pre-release development. The first tagged release will be `0.1.0` once the E2E example (`make run-example`) is verified to pass on a clean container build via `make docker-verify`.

### Architecture summary

The platform is a three-service architecture for executing programmable nodes:

- **Control plane** (`quark-server/`) — Go + Fiber. REST API, deploy orchestration, process management. Single binary, ~13 MB, <50 ms startup. No TypeScript parsing.
- **Catalog service** (`quark-catalog/`) — Go + SQLite. Stores systems, nodes, events, source files, and node packages. Pure Go (no CGO), 15 MB binary.
- **Data plane** (`quark-runtime/`) — Java + GraalJS/Truffle. Executes node systems. Parses `.quark.ts` source via GraalJS ESM evaluation. Native binary: 194 MB, 38 ms startup, includes GraalJS via `--macro:truffle-svm`.
- **CLI** (`quark-cli/`) — Go + Cobra. Operator tool (`quarkctl`) for deploy, query, watch, and node management.
- **Standard node library** (`quark-nodes/`) — Reference node implementations across 10 domains (time, system, io, stream, log, codec, data, route, net, ...).

All services communicate via an external NATS broker. Multi-tenancy is enforced by NATS subject encoding — two tenants can deploy same-named systems simultaneously with zero data leakage.
