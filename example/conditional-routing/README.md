# Conditional Routing Example

Demonstrates content-based routing with the conditional router node:

```
timer → memory-profiler → conditional-router
  ├── "high" (heap > 50%) → stdout-logger + SSE stream
  └── "normal" (everything else) → stdout-logger + SSE stream
```

## Nodes used

| Node | URI | Language |
|------|-----|----------|
| Timer | `quark/time/schedule/timer:v1` | Java |
| Memory Profiler | `quark/system/memory/profile:v1` | Java |
| Conditional Router | `quark/route/flow/conditional:v1` | TypeScript |
| Stdout Logger | `quark/log/console/stdout:v1` | TypeScript |
| SSE Broadcast | `quark/stream/sse/broadcast:v1` | Java |

## Deploy

```bash
quarkctl apply -f system.quark.ts -n demo
```

## Verify

```bash
# Watch the data plane log for routed messages
tail -f $QUARK_STATE_ROOT/dataplane-logs/dataplane-shared.log

# Connect to the SSE stream
curl -N http://localhost:8081/stream/demo/conditional-routing/stream
```

## Undeploy

```bash
quarkctl delete system conditional-routing -n demo
```
