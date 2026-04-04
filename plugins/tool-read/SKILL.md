# tool-read

Read regular text files with optional line range support.

## Usage

### CLI mode

```bash
read run --path ./notes.txt
read run --path ./app.py --start-line 10 --end-line 20
```

### HTTP server mode

```bash
read serve --addr 127.0.0.1:8092
```

POST to `/read` with body:

```json
{"path": "./app.py", "start_line": 10, "end_line": 20}
```

Returns:

```json
{"path": "./app.py", "content": "...", "total_lines": 100, "error": null}
```

## Best practice

- Always read files before writing to their line ranges
- Partial reads are faster for large files
- Check `error` field before using `content`
