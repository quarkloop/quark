# quark/codec/json/parse:v1

Parses a JSON string from the message payload into an object.

## Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `field` | string | no | `data` | Field name containing the JSON string |
| `strict` | boolean | no | `false` | Throw on parse errors instead of emitting error event |

## Events

| Event | Payload | Description |
|-------|---------|-------------|
| `parsed` | `{ data, source }` | Successfully parsed JSON object |
| `error` | `{ error, input, source }` | Parse failed (only when strict=false) |

## Example

```typescript
nodes: {
    parser: {
        uses: "quark/codec/json/parse:v1",
        listens: ["webhook.received"],
        events: ["parsed", "error"],
    }
}
```
