# tool-bash

Execute shell commands in a controlled subprocess.

## Usage

### CLI mode

```bash
bash run --cmd "ls -la"
```

### HTTP server mode

```bash
bash serve --addr 127.0.0.1:8091
```

POST to `/run` with body:

```json
{"cmd": "ls -la"}
```

Returns:

```json
{"output": "...", "exit_code": 0}
```

## Safety

- Commands run via `bash -c`
- Output is captured (stdout + stderr combined)
- Exit code is propagated
- No built-in sandbox — use Quarkfile permissions to restrict access
