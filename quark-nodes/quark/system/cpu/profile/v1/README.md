# quark/system/cpu/profile:v1

Reads CPU usage (system + process) on receipt of a trigger message.

## Config

No configuration required.

## Events

| Event | Payload | Description |
|-------|---------|-------------|
| `data` | `{ cpu, processCpu, availableProcessors, timestamp, trigger }` | CPU metrics |

## Example

```typescript
nodes: {
    cpu: {
        uses: "quark/system/cpu/profile:v1",
        listens: ["timer.tick"],
        events: ["data"],
    }
}
```
