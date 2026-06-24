# quark/route/flow/conditional:v1

Routes messages to different events based on content predicates.
First matching rule wins. If no rule matches, the message is dropped.

## Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `rules` | array | yes | — | Array of `{ when, emit }` objects |

### Rule format

```json
{
    "when": "payload.level === 'error'",
    "emit": "error"
}
```

The `when` expression is evaluated with `payload` as the context variable.
Use standard JavaScript comparison operators.

## Events

Events are dynamic — determined by the `emit` field of each rule.

## Example

```typescript
nodes: {
    router: {
        uses: "quark/route/flow/conditional:v1",
        rules: [
            { when: "payload.level === 'error'", emit: "error" },
            { when: "payload.level === 'warn'", emit: "warning" },
            { when: "true", emit: "info" }  // catch-all
        ],
        listens: ["log.received"],
        events: ["error", "warning", "info"],
    }
}
```
