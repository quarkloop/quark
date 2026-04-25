# tool-fs

Filesystem operations for agent use. Replaces the legacy `read` and `write` tools.

## Commands

- `fs read <path> [--start-line N] [--end-line N] --json` — Read file contents, optionally with line range
- `fs write <path> <content> --json` — Write content to file (overwrite)
- `fs append <path> <content> --json` — Append content to file
- `fs replace <path> <find> <replace-with> --json` — Replace all occurrences of text
- `fs list [path] --json` — List directory (default: current directory)
- `fs stat <path> --json` — Get file metadata
- `fs rm <path> --json` — Remove file or directory

## HTTP

All commands map to `POST /<command>` with JSON body.

## Important

- `write` overwrites existing files without warning
- `rm` removes files and directories permanently
- `read` supports `--start-line` and `--end-line` for partial reads (1-based, inclusive)
- Use absolute paths when possible
