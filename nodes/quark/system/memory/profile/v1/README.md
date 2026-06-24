# quark/system/memory/profile:v1

Reads JVM heap/non-heap memory usage on receipt of a trigger message.

## Config

No configuration required.

## Events

| Event | Payload | Description |
|-------|---------|-------------|
| `data` | `{ heapUsed, heapCommitted, heapMax, nonHeapUsed, nonHeapCommitted, timestamp, trigger }` | Memory metrics |

## Example

```typescript
nodes: {
    memory: {
        uses: "quark/system/memory/profile:v1",
        listens: ["timer.tick"],
        events: ["data"],
    }
}
```
