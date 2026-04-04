# tool-web-search

Search the web via Brave Search or SerpAPI. Falls back to stub results if no API key is set.

## Usage

### CLI mode

```bash
web-search run --query "golang concurrency patterns" --max-results 5
```

### HTTP server mode

```bash
web-search serve --addr 127.0.0.1:8090
```

POST to `/search`:

```json
{"query": "...", "max_results": 5}
```

Returns:

```json
{
  "results": [
    {"title": "...", "url": "...", "snippet": "..."}
  ]
}
```

## Configuration

Set one of these environment variables for real results:

- `BRAVE_API_KEY` — Brave Search API (https://api.search.brave.com)
- `SERPAPI_KEY` — SerpAPI (https://serpapi.com)

Without an API key, stub results are returned (useful for testing).