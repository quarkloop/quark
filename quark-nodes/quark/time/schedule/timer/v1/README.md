# quark/time/schedule/timer:v1

Emits a `tick` event at a fixed interval.

## Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `interval` | string | no | `1s` | Duration: `1s`, `500ms`, `10s`, `1m`, `1h` |

## Events

| Event | Payload | Description |
|-------|---------|-------------|
| `tick` | `{ tick, timestamp }` | Emitted at each interval |

## Example

```typescript
nodes: {
    timer: {
        uses: "quark/time/schedule/timer:v1",
        interval: "1s",
        events: ["tick"],
    }
}
```
