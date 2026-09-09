# GitHub Repository Conventions

## Default branch

Use `main` as the protected integration branch.

Recommended branch protection:

- require pull requests before merge
- require the CI workflow to pass
- require branches to be up to date before merge when practical
- prevent force pushes to `main`
- prevent tag rewriting for released versions

## Issues and pull requests

The repository includes structured bug/feature issue forms and a pull-request
template. Keep bug reports reproducible and architecture proposals tied to a
specific problem rather than adding technology for its own sake.

## Dependency updates

`.github/dependabot.yml` watches Go modules, npm packages, GitHub Actions,
Docker references, and Terraform providers. Dependency changes should remain separate from major feature
changes when possible so failures are easier to attribute.

## Documentation

`scripts/check-docs.py` validates core documents, relative Markdown links, ADR
numbering, and accidental local artifact paths. CI runs the same check through
`make docs-check`.

## Releases

Tags are immutable. See [the release process](release-process.md).
