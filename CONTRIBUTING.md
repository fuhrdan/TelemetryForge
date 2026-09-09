# Contributing

## Engineering standards

- Run `make check` before committing.
- Add tests for behavior changes.
- Document exported Go identifiers with GoDoc comments.
- Explain *why* when concurrency, reliability, performance, or security logic is non-obvious.
- Add or update an ADR when a significant architectural decision changes.
- Update `CHANGELOG.md` for user-visible or operationally meaningful changes.

## Commit style

Use conventional commits where practical:

- `feat: add event ingestion endpoint`
- `fix: reject malformed event envelopes`
- `test: cover metric validation`
- `docs: add ingestion architecture ADR`
- `chore: configure CI checks`
