# quark/stream/sse/broadcast:v1

Exposes an HTTP SSE endpoint that streams incoming NATS messages to connected clients.

## Config

No node-level config. HTTP server config is read from MicroProfile Config:
- `quark.streaming.host` (default `0.0.0.0`)
- `quark.streaming.port` (default `8081`)
- `quark.streaming.path-prefix` (default `stream`)

URL pattern: `/<prefix>/<namespace>/<system>/<node>`

## Events

This node does not publish events — it forwards incoming messages to SSE clients.

## Example

```typescript
nodes: {
    stream: {
        uses: "quark/stream/sse/broadcast:v1",
        listens: ["cpu.data", "memory.data"],
    }
}
```
