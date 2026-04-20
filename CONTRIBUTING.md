# Contributing

## Scope

pgxporter is deliberately narrow — a pgx-native Prometheus exporter library for PostgreSQL 13–18. We happily accept contributions that:

- Add coverage for a PG view we don't yet scrape (file a Linear ticket or open an issue first so we don't duplicate work).
- Harden an existing collector — NULL safety, version gates for new PG minor releases, correctness fixes.
- Improve the declarative `CollectorSpec` surface.
- Improve documentation, particularly around cloud-IAM-auth setup for GCP CloudSQL or Azure Database (we only ship AWS RDS today; the `AuthProvider` interface is explicitly pluggable).

Things we probably won't accept without prior discussion:

- Support for PG < 13. Our tested matrix is PG 13.22 → 18.0. Adding older versions requires reconciling schema differences we've deliberately dropped.
- A bundled HTTP server. The library stays framework-agnostic; TLS + basic auth is handled via `prometheus/exporter-toolkit` in consumer code.
- Features that require a CLA. We have a DCO-by-implicit-assent model (see below).

## Development

Requires Go 1.25+.

```bash
git clone https://github.com/becomeliminal/pgxporter.git
cd pgxporter
go build ./...
go vet ./...
go test ./...
```

Integration tests against the real Postgres matrix (PG 13.22 / 14.19 / 15.14 / 16.10 / 17.6 / 18.0):

```bash
go test -tags=integration ./...
```

Integration tests use the [`becomeliminal/postgres`](https://github.com/becomeliminal/postgres) plz plugin to spin up pre-built Postgres binaries on ephemeral ports — no Docker required. The `testutil.StartPG(t, "17.6")` helper hides the setup.

## Commit + PR conventions

- Branches: `LIM-NNNN-short-description` where `LIM-NNNN` is the Linear ticket ID, or `<handle>-short-description` for external contributors without a ticket.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `feat(scope): ...`, `fix(scope): ...`, `docs(scope): ...`, `chore: ...`. Scopes roughly match package names: `exporter`, `collectors`, `db`, `docs`, `ci`.
- Every PR must update `CHANGELOG.md` under `## [Unreleased]` in the appropriate section (Added / Changed / Fixed / Removed / Security).
- PR descriptions cite the Linear ticket ID if applicable.

## Versioning policy

Starting with v1.0.0 this project adheres strictly to [Semantic Versioning](https://semver.org/):

- **Major** bump — any backward-incompatible change to the public Go API (anything exported under `github.com/becomeliminal/pgxporter/...`) or a breaking change to the `CollectorSpec` YAML schema.
- **Minor** bump — new collectors, new `AuthProvider` implementations, new `Opts` fields, new self-metrics, any additive change that existing code doesn't have to opt into.
- **Patch** bump — bug fixes, dep refreshes, documentation improvements, schema-drift fixes when PG releases minor versions.

Pre-1.0 releases (v0.1.0, v0.2.0, v0.3.0) treated breaking changes as minor bumps and are not covered by the above contract.

## Release process

1. **Move `## [Unreleased]` to `## [X.Y.Z] — YYYY-MM-DD`** in `CHANGELOG.md`, and add a fresh empty `## [Unreleased]` above it. Update the link-reference footnotes at the bottom of the file.
2. **Commit** that as `chore(release): vX.Y.Z`.
3. **Tag and push**:

   ```bash
   git tag -a v1.0.0 -m "v1.0.0"
   git push origin main v1.0.0
   ```

The `release.yaml` GitHub Actions workflow (runs on every `v*` tag push):

1. Extracts the `## [X.Y.Z]` section from `CHANGELOG.md` as release notes. Fails loudly if the section is missing — forces step 1 above to happen before tagging.
2. Creates a **draft** GitHub Release with those notes so a maintainer can sanity-check before publishing. No auto-publish — intentional belt-and-braces for a v1.0.0 launch.

### What release automation does NOT do (today)

- **No binary publishing.** pgxporter is a library. `cmd/main.go` is a minimal reference CLI used by the head-to-head benchmark (and available as a starting point for `go build ./cmd`), but we don't publish release binaries for it yet. Consumers typically compose their own `main.go` against the library with richer flag handling, TLS via exporter-toolkit, and service-discovery glue — see the README examples. Binary / container publishing waits until that reference CLI grows out of "minimal" and into "per-release packaged".
- **No Docker image publishing.** Same reason as binaries.
- **No SBOM / cosign signing.** Deferred until binary publishing lands.

## Reporting security issues

Do not file security issues on the public tracker. Email **security@liminal.cash** with "pgxporter" in the subject.

## License & contribution terms

By submitting a patch (via pull request, email, or any other channel) you agree that your contribution is licensed under the project's Apache-2.0 license, and that you have the right to license it under those terms (developer-certificate-of-origin-style implicit assent).

Nothing else. No CLA.
