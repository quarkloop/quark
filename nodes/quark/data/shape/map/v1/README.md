# quark/data/shape/map:v1

Declaratively maps fields from the input payload to a new output shape.

## Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `mapping` | object | yes | — | Source path → target field mapping |
| `preserve` | boolean | no | `false` | Include unmapped source fields in output |

## Events

| Event | Payload | Description |
|-------|---------|-------------|
| `mapped` | `{ ...mappedFields, _source }` | Remapped payload |

## Example

```typescript
nodes: {
    shaper: {
        uses: "quark/data/shape/map:v1",
        mapping: { "data.cpu": "cpuUsage", "data.mem": "memUsage" },
        listens: ["profiler.data"],
        events: ["mapped"],
    }
}
```
