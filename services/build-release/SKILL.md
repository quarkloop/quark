# service-build-release

The build-release service runs Quark's Go release pipeline over gRPC. It is
independently deployable and does not depend on the agent plugin runtime.

Use `quark.buildrelease.v1.BuildReleaseService` when release automation should
run as a service instead of as a local tool plugin.

## RPCs

- `Release(ReleaseRequest) -> ReleaseResponse`
  - Required: `working_dir`
  - Optional: `config_path`, `version`, `parallelism`, `skip_tests`
  - Runs config loading, version resolution, optional tests, cross-compilation,
    archive generation, checksums, signing, README, and metadata output.

- `DryRun(DryRunRequest) -> DryRunResponse`
  - Required: `working_dir`
  - Optional: `config_path`, `version`, `parallelism`
  - Returns the artifact matrix without compiling or writing release files.

- `Init(InitRequest) -> InitResponse`
  - Required: `working_dir`
  - Optional: `overwrite`
  - Creates `build_release.json` when it does not already exist.

## Contract Notes

- Paths are resolved relative to `working_dir` unless already absolute.
- Cancellation is honored for external commands such as `go test`, `go build`,
  and `gpg`.
- The service owns the release pipeline; the legacy plugin is a compatibility
  adapter and should not contain release business logic.
