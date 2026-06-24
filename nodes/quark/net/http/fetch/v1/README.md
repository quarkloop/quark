# quark/net/http/fetch:v1

Fetches a URL from the message payload and emits the response body.

## Config

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `urlField` | string | no | `url` | Field name containing the URL to fetch |
| `method` | string | no | `GET` | HTTP method |
| `headers` | object | no | `{}` | HTTP headers |
| `timeout` | integer | no | `30000` | Request timeout in ms |

## Events

| Event | Payload | Description |
|-------|---------|-------------|
| `response` | `{ url, status, body, source }` | Successful HTTP response |
| `error` | `{ error, url, source }` | Request failed |

## Example

```typescript
nodes: {
    fetcher: {
        uses: "quark/net/http/fetch:v1",
        method: "GET",
        listens: ["scheduler.tick"],
        events: ["response", "error"],
    }
}
```
