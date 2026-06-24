# User Story — A Typical Day with Quark

**Meet Alice — a platform engineer at a company that runs multiple data pipelines.**

This is the story of how Alice uses Quark to build, deploy, monitor, and manage a log-processing pipeline — by writing a single TypeScript file.

---

## 1. Alice installs the CLI

```bash
quarkctl version
export QUARK_HOST=https://quark.internal.company.com
export QUARK_NAMESPACE=alice-team
```

---

## 2. Alice writes her program

Alice writes a single TypeScript file called `access-log-processor.quark.ts`:

```typescript


export default({
    name: "access-log-processor",
    namespace: "alice-team",

    nodes: {
        "nginx-logs": {
            uses: "source/filesystem:v1",
            path: "/var/log/nginx",
            pattern: "*.log",
            events: ["data"],
        },

        "parse-json": {
            uses: "function/json-decoder:v2",
            strict: false,
            listens: ["nginx-logs.data"],
            events: ["data"],
            onFailure: { retry: 3, routeTo: "ops-dashboard" },
        },

        "enrich-geo": {
            uses: "function/geoip-enricher:v1",
            ip_field: "client_ip",
            listens: ["parse-json.data"],
            events: ["data"],
            onFailure: { retry: 3, routeTo: "ops-dashboard" },
        },

        "classify-severity": {
            uses: "function/classifier:v1",
            field: "level",
            classes: ["error", "normal"],
            listens: ["enrich-geo.data"],
            events: ["error", "normal"],
        },

        "alert-db": {
            uses: "store/time-series:latest",
            retention: "90d",
            listens: ["classify-severity.error"],
        },

        "analytics-db": {
            uses: "store/columnar:v1",
            retention: "365d",
            listens: ["classify-severity.normal"],
        },

        "ops-dashboard": {
            uses: "endpoint/dashboard:latest",
            listens: ["classify-severity.error", "fallback.parse-json", "fallback.enrich-geo"],
        },
    },
});
```

That's it. That's her entire program. No Java, no Docker, no Kubernetes manifests — just a `.quark.ts` file declaring what nodes she needs and how they communicate through NATS subjects.

The TypeScript file IS the program. Alice never touches Java code.

---

## 3. Alice deploys

```bash
quarkctl apply -f access-log-processor.quark.ts -n alice-team
```

Output:
```
✓ System alice-team/access-log-processor created.
  Nodes:     7
  State:     ACTIVE
  Health:    HEALTHY
```

Behind the scenes:
1. CLI PUTs the TypeScript source to the control plane (`PUT /api/v1/namespaces/alice-team/systems/access-log-processor`).
2. Control plane parses the source with `SimpleSystemParser` (regex-based, no GraalJS) → `SystemDefinition`.
3. Control plane persists the system record + source to the Catalog via NATS (`catalog.system.save`, `catalog.source.save`).
4. Control plane's `ProcessManager` ensures a data-plane process exists for the runtime (spawns one if needed).
5. Control plane sends a `DeployCommand` to `quark.control.<runtimeId>.deploy` via NATS request-reply.
6. Data plane receives the command, parses the source with `GraalJsSystemParser` (full GraalJS), instantiates providers, creates NATS subscriptions for each node's `listens`, validates each node's `events` against its declared publish set, and starts the lifecycle.
7. Data plane replies with a `StatusResponse` containing node info.
8. Control plane returns `201 Created` to the CLI.

---

## 4. Alice monitors

```bash
quarkctl get systems -n alice-team
quarkctl get nodes -n alice-team -s access-log-processor
quarkctl watch events -n alice-team -s access-log-processor
quarkctl get node classify-severity -n alice-team -s access-log-processor
```

---

## 5. Alice pauses a node

> **Note (v8 status):** The `pause`/`resume`/`drain`/`archive`/`recover` node lifecycle commands are **not yet implemented** in the current CLI. The lifecycle state machine exists in the engine (`CREATING → ACTIVE → PAUSED → DRAINING → ARCHIVED → DELETED`), but there are no CLI commands or REST endpoints to trigger transitions beyond deploy/undeploy. This section describes the intended UX.

```bash
quarkctl node pause enrich-geo -n alice-team -s access-log-processor   # (planned, not yet implemented)
```

In the intended design, the NATS subscription for `enrich-geo` would pause. With NATS Core (current), messages would be lost while paused; with the planned JetStream upgrade, messages would accumulate and resume on `node resume`.

```bash
quarkctl node resume enrich-geo -n alice-team -s access-log-processor  # (planned, not yet implemented)
```

---

## 6. Bob's team deploys the same thing — completely isolated

```bash
quarkctl apply -f access-log-processor.quark.ts -n bob-team
```

Bob gets his own NATS subjects: `access-log-processor.bob-team.*`. Alice and Bob are fully isolated — different subjects, different node instances, different data-plane metrics heartbeats. If both use the default `runtime: "shared"`, they share the same data-plane process but remain isolated at the NATS subject level. If Bob adds `runtime: "isolated"` to his `.quark.ts`, he gets a dedicated data-plane process (`runtimeId=ns-bob-team`).

---

## 7. Alice updates her system

Alice edits her TypeScript file to add a policy node and redeploys:

```bash
quarkctl apply -f access-log-processor.quark.ts -n alice-team
```

The control plane sends an `undeploy` command to the data plane (which stops the old system's providers and unsubscribes its NATS subjects), then sends a `deploy` command for the new version. The Catalog's system record is updated with the new source.

---

## 8. An AI agent monitors for Alice

```bash
quarkctl get system access-log-processor -n alice-team --json
```

```json
{
  "systemName": "access-log-processor",
  "namespace": "alice-team",
  "overall": "HEALTHY",
  "nodeCount": 8,
  "perNode": {
    "nginx-logs": "HEALTHY",
    "parse-json": "HEALTHY",
    ...
  }
}
```

---

## 9. Alice undeploys

```bash
quarkctl delete system access-log-processor -n alice-team
```

Control plane sends an `undeploy` command to the data plane (which stops all providers and unsubscribes NATS subjects), then deletes the system record and source from the Catalog. Clean.

---

## Summary — The Quark Principle

| Concept | Kubernetes | Quark |
|---------|-----------|-------|
| The program | `deployment.yaml` | `.quark.ts` |
| What the user writes | YAML only | TypeScript only |
| What the user runs | `quarkctl apply -f` | `quarkctl apply -f` |
| What the user monitors | `quarkctl get systems` | `quarkctl get nodes` |
| Communication | Kubernetes Services + networking | NATS Core subjects (JetStream planned) |
| Multi-tenancy | namespaces | namespaces |
| AI agent integration | `quarkctl ... --json` | `quarkctl ... --json` |

**The `.quark.ts` file IS the program.** The CLI is the interface. The control plane is the interpreter (parses + orchestrates). The Catalog persists metadata. The data plane executes (GraalJS + providers). NATS is the backbone. Users never write Java code.
