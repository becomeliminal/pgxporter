# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and starting with v1.0.0 this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Pre-1.0 releases treated breaking changes as minor bumps.

## [Unreleased]

### Fixed

- **`db.Client.AtLeast` now lazily re-probes `server_version_num` when the cached value is zero.** The initial probe in `db.New` could race a postgres server that was still starting up; on probe failure the client cached `ServerVersionNum=0` for the remainder of its lifetime, which made every version-gated SQL fall back to the pre-PG-17 column set and fail forever on PG 17 servers (`column "checkpoints_timed" does not exist`). `AtLeast` now retries the probe on demand under a short timeout, serialised by a `probeMu` mutex so a fan-out scrape doesn't issue concurrent version queries. A failed re-probe logs a warning and returns false; the next call retries. Existing tests using bare `&Client{}` literals continue to work — the re-probe is skipped when the pool is nil.

## [1.0.0-rc1] — 2026-04-20

First public release candidate for pgxporter. Targets parity with the 80/20 of `prometheus-community/postgres_exporter` on top of a pgx v5 / scany v2 foundation, with cloud IAM auth, a declarative collector extension path, and three first-mover collectors (`pg_stat_io`, `pg_stat_slru`, `pg_stat_ssl`, `pg_stat_subscription`) as the headline differentiators. 27 collectors total (24 default-on). GA (`v1.0.0`) follows a dogfood window.

### Changed from the v0.3.0 baseline

- **Collectors use stateful `*prometheus.CounterVec` / `*prometheus.GaugeVec` children instead of per-emission `MustNewConstMetric`.** Every collector (plus the declarative `SpecCollector` runner) now registers its metrics at construction time and mutates cached children on each scrape. Per-scrape allocations drop from ~10,700 to ~1,600 (−85%); per-scrape bytes drop from ~530 KB to ~207 KB (−60%); wall time improves ~18%. Counter semantics are preserved via a `counterDelta` helper that tracks last-observed absolute values per label combo and calls `Add(delta)`, handling counter resets via `DeleteLabelValues` + fresh Add so Prometheus's `rate()` / `increase()` reset detection still works. No user-facing API change.

### Added

#### Collectors

- `pg_stat_database` — xact, block IO, tuple, conflict, deadlock counters.
- `pg_stat_bgwriter` with PG 17 split handling.
- `pg_stat_checkpointer` — PG 17+ new view.
- `pg_stat_archiver` — WAL-archive monitoring.
- `pg_stat_replication` — primary-side replica lag.
- `pg_stat_wal_receiver` — standby-side.
- `pg_replication_slots` — slot retention + status.
- `pg_database_size` via `pg_database_size()`.
- `pg_stat_wal` — PG 14+ WAL write stats.
- `pg_stat_io` — PG 16+ per-backend-type I/O; postgres_exporter has no equivalent.
- `pg_stat_slru` — PG 13+ SLRU cache stats; postgres_exporter has no equivalent.
- `pg_stat_progress_vacuum` + `analyze` / `basebackup` / `copy` / `create_index` / `cluster` — full progress suite.
- Full-fidelity `pg_stat_activity` rewrite with `wait_event_type`, `wait_event`, `backend_type`, `application_name`, `state` labels.
- `pg_locks` rewrite with `pg_blocking_pids()` blocking-chain detection.
- `pg_stat_user_tables` — added PG 13–17 columns `last_seq_scan`, `n_tup_newpage_upd`, `n_ins_since_vacuum` with runtime version gating.
- `pg_stat_ssl` — aggregate TLS connection counts grouped by `(ssl, version, cipher, bits)`. Default on. postgres_exporter has no equivalent.
- `pg_stat_subscription` (+ `pg_stat_subscription_stats` on PG 15+) — logical-replication subscriber state, aggregated per subname. Default off. postgres_exporter has no equivalent.
- `pg_settings` — numeric-typed subset of `pg_settings` (`bool`, `integer`, `real`), with bools mapped to 0/1 server-side. Default off (cardinality-bounded opt-in).

#### Cloud & auth

- `db.AuthProvider` interface — `Password(ctx, host, port, user) (string, error)` — for cloud-IAM auth via pgx v5's `BeforeConnect` hook.
- `awsrds.NewDefault(ctx, region)` — AWS RDS IAM token provider wiring the default AWS credentials chain (env → shared config → IRSA → IMDS).

#### Extensibility

- Declarative `CollectorSpec` runner with version gating, label columns, and counter/gauge types.
- `Exporter.ExtendFromYAMLFile(path)` — queries.yaml replacement for runtime-loaded custom collectors.
- `Opts.MetricPrefix` + `collectors.SetMetricPrefix` — flip to `MetricPrefixPg` for postgres_exporter dashboard-name compatibility.
- `Opts.EnabledCollectors` / `Opts.DisabledCollectors` — per-collector toggles.

#### Observability of self

- `pg_stat_scrape_duration_seconds` histogram per-collector.
- `pg_stat_scrape_errors_total` counter per-collector, pre-initialised to zero so `rate()` returns a clean baseline.
- `pg_stat_metric_cardinality` gauge per-collector — early-warning for cardinality explosions.

#### Operational polish

- `Exporter.Shutdown(ctx)` — bounded graceful shutdown; drains in-flight scrapes under the mutex, closes every pgxpool.
- `Opts.CollectionTimeout` — per-scrape deadline propagated to every collector's `ctx`.

#### Foundation & tooling

- Runtime PostgreSQL version detection cached on `db.Client.ServerVersionNum` + `Client.AtLeast(major, minor)` helper.
- Prometheus scrape-context propagation end-to-end — `Collector.Scrape(ctx, ch)`.
- Real-Postgres integration test matrix across PG 13.22 / 14.19 / 15.14 / 16.10 / 17.6 / 18.0.
- Unit-test scaffolding with NULL-handling coverage per collector.
- GitHub Actions CI — build, vet, test, golangci-lint v2.
- `go test -bench` scrape benchmark + `BENCHMARKS.md`.
- `BenchmarkExporterCollectMultiDB` (1/4/16-DB fan-out scaling) and `BenchmarkExporterCollectColdVsWarm` (prepared-statement-cache on vs off). Numbers in `BENCHMARKS.md`.
- Integration matrix coverage for every collector's SELECT against PG 13.22 / 14.19 / 15.14 / 16.10 / 17.6 / 18.0 — no collector is shipped without a schema-drift guard.

#### Docs

- README rewrite with feature matrix, 24-collector table, quickstart.
- `docs/migrating-from-postgres_exporter.md` — full switch-over guide.
- Per-package godoc + `doc.go` for every package.
- README section on exporter-toolkit TLS + basic auth.

#### Benchmarks

- Head-to-head HTTP-scrape benchmark vs postgres_exporter v0.19.1. Both exporters run as subprocesses against identical fresh PG 17.6 instances with seeded fixtures, measured strictly from the client side so allocation numbers are symmetric. Representative 2026-04-20 numbers: pgxporter **8.7 ms / 2,610 series** vs postgres_exporter 20.2 ms / 1,967 series — **~2.3× faster wall time, 33% more metrics**. Full methodology + raw numbers in `benchmarks/head-to-head/`.
- Upgraded `cmd/main.go` from a 10-line stub to a minimal reference CLI — serves `/metrics` via promhttp, reads DSN from `DATA_SOURCE_NAME`, supports `--web.listen-address`, `--web.telemetry-path`, `--collection-timeout`, `--log.level`, handles SIGTERM cleanly via `Exporter.Shutdown`. Primary driver: the head-to-head benchmark now compiles and runs pgxporter as a subprocess for methodologically fair comparison against postgres_exporter's binary.

### Changed

- **Relicensed from AGPL-3.0 to Apache-2.0** — aligns with the Prometheus exporter ecosystem; removes the adoption blocker posed by AGPL §13.
- Replaced `logrus` with stdlib `log/slog`; dropped noisy per-scrape INFO logs.
- Removed latent-deadlock `Collect()` methods on per-collector mutexes. Internal change only; no user-facing API impact — but removes a footgun for anyone who registered a collector directly.

### Removed

- Unimplemented `AuthMechanism=client_certificates` stub and its `configureAuthParams` dead switch. Use `AuthProvider` for anything beyond password auth.

### Security

- Refreshed dependencies including pgx 5.9.2 (CVE patch) and Go 1.25 toolchain (#38).

## [0.3.0] — 2026-04-18

### Changed

- Migrated to pgx v5 + scany v2 with `pgtype`-everywhere row scans. Fixes a long-standing bug where partitioned-table parents under PG 17 returned NULL counters and the old `int` model type crashed every scrape. Also fixes a pre-existing `fmt.Sprintf("%w", err)` vet error that was printing `%!w(*fmt.wrapError=…)` garbage and obscuring real error messages.

## [0.2.0] — 2026-04-17

### Changed

- Module-path / packaging cleanup. Internal-only release.

## [0.1.0] — 2026-04-17

### Added

- Initial public tag. Collectors for `pg_stat_activity` (reduced to count + max transaction duration), `pg_stat_statements`, `pg_locks`, `pg_stat_user_tables`, `pg_stat_user_indexes`, `pg_statio_user_tables`, `pg_statio_user_indexes`.
- `exporter.Exporter` driving collectors in parallel via `errgroup`.
- `db.Client` wrapping pgx/pgxpool with per-DB opts.

[Unreleased]: https://github.com/becomeliminal/pgxporter/compare/v1.0.0-rc1...HEAD
[1.0.0-rc1]: https://github.com/becomeliminal/pgxporter/compare/v0.3.0...v1.0.0-rc1
[0.3.0]: https://github.com/becomeliminal/pgxporter/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/becomeliminal/pgxporter/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/becomeliminal/pgxporter/releases/tag/v0.1.0
