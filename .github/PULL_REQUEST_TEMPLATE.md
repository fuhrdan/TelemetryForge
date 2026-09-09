## Summary

Describe the problem and the change in a few sentences.

## Validation

- [ ] `make check`
- [ ] `make docs-check`
- [ ] dashboard build/type check (if applicable)
- [ ] integration tests (if external behavior changed)

## Documentation / architecture

- [ ] public behavior/configuration docs updated
- [ ] `CHANGELOG.md` updated when operationally meaningful
- [ ] ADR added or superseded when architecture changed

## Safety / data

- [ ] no credentials, secrets, private telemetry, or local artifact paths
- [ ] retry/acknowledgement/idempotency semantics reviewed if this touches the processing path
