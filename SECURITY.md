# Security Policy

## Supported versions

This project is in early development. Only the latest `main` branch receives security fixes. Once a 1.0 release is cut, we'll publish a support table here.

| Version | Supported          |
|---------|--------------------|
| main    | :white_check_mark: |
| tagged releases | :white_check_mark: (latest two minor versions) |
| older   | :x:                |

## Reporting a vulnerability

**Do NOT file a public GitHub issue for security vulnerabilities.**

Instead, email **reza.ebrahimi.dev@gmail.com** with:

1. A description of the vulnerability and its impact.
2. Steps to reproduce (a minimal reproducer is ideal).
3. Affected component (`quark-server`, `quark-runtime`, `quark-cli`, `quark-catalog`, or `quark-nodes`) and version.
4. Any suggested fixes or mitigations.

You should receive an acknowledgment within 48 hours. If you don't, please follow up to confirm we received the original report — email can get filtered.

We will coordinate disclosure with you and credit your report in the release notes unless you prefer to remain anonymous.

## Disclosure timeline

1. **Day 0**: We receive the report and confirm receipt within 48 hours.
2. **Day 0–7**: We reproduce the issue and assess severity.
3. **Day 7–30**: A fix is developed on a private branch.
4. **Day 30**: The fix is released and the vulnerability is disclosed publicly with credit to the reporter (unless anonymity is requested).

Critical vulnerabilities may be fixed and disclosed faster. Lower-severity issues may sit longer if a fix would require a breaking change.

## Scope

This security policy applies to all five components of the Quark platform: `quark-server`, `quark-runtime`, `quark-cli`, `quark-catalog`, and `quark-nodes`.

### In scope

- Authentication or authorization bypass in the control plane REST API.
- Authentication or authorization bypass in NATS subject routing (cross-tenant data leakage).
- Remote code execution via the data plane (e.g. via GraalJS sandbox escape, native node execution, or malicious `.quark.ts` source).
- Deserialization vulnerabilities in any component.
- Memory safety issues in the Java runtime or Go binaries.
- SQL injection in the Catalog's SQLite layer.
- Supply-chain risks (compromised dependencies, malicious publish artifacts).

### Out of scope

- Vulnerabilities in NATS server itself — report to [nats-io/nats-server](https://github.com/nats-io/nats-server).
- Vulnerabilities in GraalVM, Quarkus, or other upstream Java libraries — report upstream.
- Vulnerabilities in the Go standard library or Fiber / nats.go / zap — report upstream.
- Social engineering attacks against maintainers or users.
- Theoretical timing attacks without a demonstrated exploit.
- Denial of service via resource exhaustion on the NATS broker (mitigate at the broker level).

## Hardening recommendations

When deploying Quark in production:

1. **Secure the NATS broker.** Use TLS for NATS connections (`tls://` or `wss://`). Configure NATS account credentials so each tenant has isolated subject namespaces. The platform's multi-tenant model relies on NATS subject isolation; do not run with anonymous NATS access in production.
2. **Restrict the control plane REST API.** Bind `quark-server` to a private network or put it behind an authenticated reverse proxy. The REST API has no built-in auth — it assumes a trusted network.
3. **Validate `.quark.ts` source before deploy.** The data plane evaluates TypeScript via GraalJS with ESM. While GraalJS is sandboxed by default, do not deploy untrusted `.quark.ts` files without code review.
4. **Restrict Catalog SQLite access.** The Catalog uses `modernc.org/sqlite` (pure Go, no CGO). The database file should be on a filesystem with appropriate permissions; the Catalog process should run as a non-root user.
5. **Pin container images by digest.** When deploying via Docker, pin to a specific image digest, not just a tag.
6. **Monitor NATS subjects.** Set up NATS streaming / JetStream observability so you can detect unusual subject activity (e.g. a tenant trying to subscribe to another tenant's subjects).

## Trust boundaries

```
                 ┌─────────────────────────────────────────────┐
                 │  Trusted zone (your deployment)             │
   Operator  ────┤                                             │
                 │  quark-server (no built-in auth)            │
                 │  quark-runtime (GraalJS sandbox)            │
                 │  quark-catalog (SQLite, no remote access)   │
                 │  NATS broker (must be TLS + auth)           │
                 └─────────────────────────────────────────────┘
                                     │
                                     ▼
                 ┌─────────────────────────────────────────────┐
                 │  Untrusted zone                             │
                 │  .quark.ts source files (review before      │
                 │  deploy — GraalJS sandbox contains them but │
                 │  defense in depth is still recommended)     │
                 │  Untrusted tenant workloads (rely on NATS   │
                 │  subject isolation)                         │
                 └─────────────────────────────────────────────┘
```
