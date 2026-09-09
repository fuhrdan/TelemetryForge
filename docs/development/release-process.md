# Release Process

TelemetryForge tags are intended to be immutable snapshots.

## Before a release

1. Update the release section in `CHANGELOG.md`.
2. Update `RELEASE_NOTES.md` for the release being cut.
3. Update `README.md`, `ROADMAP.md`, and the documentation index if capabilities
   or navigation changed.
4. Add or update ADRs for architecture changes.
5. Run:

```bash
make docs-check
make check
make policy-check
make integration-test
make dashboard-build
make k8s-render
make terraform-check
make observability-check
```

6. Run the complete Docker Compose stack and the healthy/incident demos.
7. Confirm no development credentials or local artifact paths were committed.
8. Commit the release with a conventional commit message.
9. Create an annotated or lightweight semantic-version tag such as `v0.9.0`.

## Development branches

Do not modify an already tagged release in place. New work should branch from
the latest stable tag/commit and remain untagged until its acceptance criteria
are met.

The current repository follows this rule: `v0.9.0` branches from the immutable
`v0.8.0` release. The older `v0.7.0-dev` branch remains a development-history
point rather than a release tag.

## Versioning

Pre-1.0 releases use semantic versioning as an engineering roadmap:

- minor version: coherent new capability milestone
- patch version: compatible bug/documentation/security hardening that should be
  released independently

## GitHub release notes

When publishing a GitHub Release, use `RELEASE_NOTES.md` as the human-readable
body, attach only artifacts that were produced from the tagged commit, and
avoid claiming benchmarks that are not reproducible from the repository.
