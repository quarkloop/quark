# tool-build-release

Cross-compile Go binaries, generate checksums, and produce install scripts.

## Commands

- `build-release release [config] [--version V] [--parallel N] [--skip-tests] --json` — Full release pipeline
- `build-release dryrun [config] [--version V] --json` — Preview without compiling
- `build-release init --json` — Scaffold build_release.json

## Important

- Requires `go` in PATH
- `release` runs: config → version → validate → test → build → archive → checksum → sign → readme → metadata
- `dryrun` only runs: config → version → validate → build (dry)
- Config file defaults to `build_release.json` in working directory
