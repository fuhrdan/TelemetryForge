# Technology Version Baseline

Verified for the v0.7.0 development baseline on **2026-09-09**.

| Component | Project baseline | Rationale |
|---|---:|---|
| [Go](https://go.dev/doc/devel/release) | 1.27.1 | Current supported Go patch release |
| [Apache Kafka](https://kafka.apache.org/community/downloads/) | 4.3.1 | Current supported Apache Kafka release |
| [franz-go](https://pkg.go.dev/github.com/twmb/franz-go) | 1.21.6 | Pinned Kafka client used by the current tested code path |
| [pgx](https://github.com/jackc/pgx/blob/master/CHANGELOG.md) | 5.10.0 | Current project PostgreSQL client baseline |
| [TimescaleDB](https://github.com/timescale/timescaledb/releases) | 2.30.0 / PostgreSQL 17 image | Current TimescaleDB release |
| [Node.js](https://nodejs.org/en/download) | 24.21.0 LTS | LTS runtime for the dashboard |
| [Next.js](https://nextjs.org/docs) | 16.3.4 | Current Next.js release |
| [React / React DOM](https://www.npmjs.com/package/react) | 19.2.8 | Current npm stable packages |
| [TypeScript](https://www.npmjs.com/package/typescript) | 5.9.3 | Deliberately held below 7.0 because stable Next.js still relies on the legacy JavaScript compiler API |
| [Alpine Linux](https://www.alpinelinux.org/) | 3.24.1 | Current stable Alpine 3.24 patch release |
| GitHub Actions | checkout/setup-go/setup-node v7 | Current action generation for GitHub-hosted runners |


## Dashboard container note

Node.js 24.21.0 is the current LTS used by CI and recommended for direct
development. During this audit, Docker Hub's latest **verified exact** Node 24
Alpine 3.24 tag was still `node:24.20.0-alpine3.24`.

The dashboard container therefore stays on that known-good exact tag instead of
referencing an unverified `24.21.0` image. Once Docker Hub publishes/verifies the
matching exact tag, update the Dockerfile in a normal dependency pull request.

## Dependency policy

Core runtime versions are pinned deliberately rather than floating on `latest`.
Minor/patch upgrades should be made through reviewed pull requests, preferably
with Dependabot, and should pass unit, integration, dashboard, and container
CI before merge.

TypeScript 7.0.2 is the current stable TypeScript release, but it is a native
Go-based compiler package and no longer exposes the legacy
`typescript/lib/typescript.js` JavaScript Compiler API that stable Next.js
builds still expect. The project therefore keeps the dashboard on TypeScript
5.9.3 for a predictable `next build` path during v0.7.0 development.

This is an explicit compatibility pin, not an assumption that 5.9.3 is current.
It should be revisited once the project's selected stable Next.js release can
run its normal production build against TypeScript 7 without an experimental
compatibility path.

The npm dashboard currently uses exact direct dependency versions. A generated
`package-lock.json` should be committed from the first successful registry-backed
`npm install` on the v0.7.0 branch so transitive dependencies are reproducible as
well.
