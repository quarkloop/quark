## Description

<!-- What does this PR do and why? Link related issues with "Closes #N". -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor
- [ ] Documentation
- [ ] Chore / dependency update

## Checklist

- [ ] `make build` passes (Go binaries + Java modules)
- [ ] `make test` passes (Go + Java unit tests)
- [ ] `make arch-check` passes when architecture or package ownership changes
- [ ] Services do not call each other directly (communication via NATS only)
- [ ] No TypeScript parsing added to the control plane (control plane treats `.quark.ts` as opaque)
- [ ] GraalJS changes are scoped to the data plane only
- [ ] Relevant documentation updated (README, AGENTS.md, `docs/*.mdx`)
- [ ] Changes are scoped — no unrelated files in this PR
