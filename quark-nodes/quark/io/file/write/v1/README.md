# quark/io/file/write:v1

Appends incoming messages as JSON Lines to a file on disk.

## Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `path` | string | yes | — | File path |
| `mode` | string | no | `append` | Write mode |

## Events

This node does not publish events — it is a terminal consumer.

## Example

```typescript
nodes: {
    writer: {
        uses: "quark/io/file/write:v1",
        path: "./output.jsonl",
        mode: "append",
        listens: ["cpu.data", "memory.data"],
    }
}
```
