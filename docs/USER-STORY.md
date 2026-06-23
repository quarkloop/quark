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
quarkctl system deploy -f access-log-processor.quark.ts -n alice-team
```

Output:
```
✓ System alice-team/access-log-processor deployed.
  Nodes:     7
  State:     DEPLOYED
  Health:    HEALTHY
```

Behind the scenes:
1. CLI sends the TypeScript source to the server
2. Server transpiles TS → JS, evaluates via GraalJS
3. Server reads the system config
4. Server resolves every node URI against the provider registry
5. Server creates NATS JetStream stream for `access-log-processor.alice-team.>`
6. Server creates JetStream consumers for each node's `listens`
7. Server creates publish ACLs for each node's `events`
8. Server instantiates and starts all SPI providers
9. Server persists the source file and state snapshot

---

## 4. Alice monitors

```bash
quarkctl system list -n alice-team
quarkctl node list -n alice-team -s access-log-processor
quarkctl event watch -n alice-team -s access-log-processor
quarkctl health node classify-severity -n alice-team -s access-log-processor
```

---

## 5. Alice pauses a node

```bash
quarkctl node pause enrich-geo -n alice-team -s access-log-processor
```

The NATS consumer for `enrich-geo` pauses. Messages accumulate in JetStream. On resume, the consumer picks up where it left off.

```bash
quarkctl node resume enrich-geo -n alice-team -s access-log-processor
```

---

## 6. Bob's team deploys the same thing — completely isolated

```bash
quarkctl system deploy -f access-log-processor.quark.ts -n bob-team
```

Bob gets his own NATS subjects: `access-log-processor.bob-team.*`. Alice and Bob are fully isolated — different JetStream streams, different consumers, different subjects.

---

## 7. Alice updates her system

Alice edits her TypeScript file to add a policy node and redeploys:

```bash
quarkctl system deploy -f access-log-processor.quark.ts -n alice-team
```

The server undeploys the old system and deploys the new one. NATS stream is recreated.

---

## 8. An AI agent monitors for Alice

```bash
quarkctl health system access-log-processor -n alice-team --json
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
quarkctl system delete access-log-processor -n alice-team
```

Server stops all consumers, removes NATS stream, deletes persistent state. Clean.

---

## Summary — The Quark Principle

| Concept | Kubernetes | Quark |
|---------|-----------|-------|
| The program | `deployment.yaml` | `.quark.ts` |
| What the user writes | YAML only | TypeScript only |
| What the user runs | `quarkctl apply -f` | `quarkctl system deploy -f` |
| What the user monitors | `quarkctl get systems` | `quarkctl node list` |
| Communication | Kubernetes Services + networking | NATS JetStream subjects |
| Multi-tenancy | namespaces | namespaces |
| AI agent integration | `quarkctl ... --json` | `quarkctl ... --json` |

**The `.quark.ts` file IS the program.** The CLI is the interface. The server is the interpreter. NATS is the backbone. Users never write Java code.
