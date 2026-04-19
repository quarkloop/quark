# tool-write

Write and edit regular text files.

## Usage

### CLI mode

```bash
# Create/write
write run --path ./notes.txt --content "hello"

# String replace
write run --path ./app.py --operation replace --find "foo" --replace-with "bar"

# Line-edit (precise)
write run --path ./app.py --operation edit \
  --start-line 2 --start-column 1 --end-line 2 --end-column 14 \
  --new-text "print('hi')"
```

### HTTP server mode

```bash
write serve --addr 127.0.0.1:8092
```

Operations: `write`, `replace`, `edit`.

## Best practice

- Use `edit` with line/column ranges for surgical changes
- Use `write` for creating new files
- Use `replace` for find-and-replace across a file
- Always read the file first to get accurate line numbers