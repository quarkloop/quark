# quark/log/console/stdout:v1

Writes each incoming message payload to standard output as JSON.

## Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `pretty` | boolean | no | `false` | Pretty-print JSON output |

## Events

This node does not publish any events — it is a terminal consumer.

## Example

```typescript
nodes: {
    logger: {
        uses: "quark/log/console/stdout:v1",
        listens: ["timer.tick"],
    }
}
```
