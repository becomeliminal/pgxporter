# Grafana dashboards

Two paths to a working pgxporter dashboard in Grafana:

1. **Reuse existing postgres_exporter community dashboards** — zero custom JSON. Requires `MetricPrefix = MetricPrefixPg` so metric names line up. Covers cluster/database/table metrics.
2. **Install the pgxporter-native dashboards in this directory** — covers the parts postgres_exporter can't: exporter self-metrics, plus any future `pg_stat_io` / `pg_stat_slru` / `pg_stat_subscription` / `pg_stat_ssl` coverage.

Most deployments want both.

## Reusing postgres_exporter dashboards

pgxporter defaults to the `pg_stat_*` namespace (e.g. `pg_stat_database_xact_commit_total`). postgres_exporter uses `pg_*` (e.g. `pg_database_xact_commit_total`). Flip to the compat prefix in your pgxporter config:

```go
import "github.com/becomeliminal/pgxporter/exporter/collectors"

exp, err := exporter.New(ctx, exporter.Opts{
	MetricPrefix: collectors.MetricPrefixPg,
	DBOpts:       []db.Opts{…},
})
```

With that set, these community dashboards import and render without further changes:

| Dashboard | Grafana.com ID | What it covers |
| --- | --- | --- |
| [PostgreSQL Exporter](https://grafana.com/grafana/dashboards/9628) | 9628 | Cluster-level overview — connections, locks, cache hit rate, background writer, replication lag |
| [PostgreSQL Database](https://grafana.com/grafana/dashboards/12485) | 12485 | Per-database breakdowns — transactions, sessions, temp files, conflict/deadlock counts |

Import via Grafana UI (`+ → Import → Grafana.com ID`). Set the Prometheus datasource when prompted. If a panel shows no data, check metric name in Prometheus explorer — usually the fix is confirming `MetricPrefix = MetricPrefixPg` is actually taking effect.

### Known mismatches

A handful of panels on community dashboards reference metric names that differ from pgxporter even under compat prefix. These are documented in [`docs/migrating-from-postgres_exporter.md`](../docs/migrating-from-postgres_exporter.md#known-metric-name-deltas). The biggest one: counter names in pgxporter end in `_total` (Prometheus convention); postgres_exporter dropped the suffix. Grafana panels that use `rate()` or `increase()` on those counters work fine in both; panels that name the counter bare don't.

## pgxporter-native dashboards

### pgxporter-health.json

The exporter's own observability. Every pgxporter deployment should have this — community postgres_exporter dashboards don't cover our self-metrics, and our self-metrics are strictly more detailed (per-collector histograms + error counts + cardinality gauges).

Panels:

- **`pg_stat_up`** per exporter — binary up/down.
- **Scrape rate** — `rate(pg_stat_exporter_scrapes_total[1m])`.
- **Per-collector scrape P95** — `histogram_quantile(0.95, sum by (le, collector) (rate(pg_stat_scrape_duration_seconds_bucket[5m])))`. Flags slow collectors before they blow the scrape timeout.
- **Per-collector error rate** — `sum by (collector) (rate(pg_stat_scrape_errors_total[5m]))`. Surfaces schema drift or permission issues per collector.
- **Per-collector cardinality** — `pg_stat_metric_cardinality`. Early warning for cardinality explosions (high-cardinality `application_name` on `pg_stat_activity`, etc.).

Import: Grafana UI → `+ → Import → Upload JSON file → pgxporter-health.json`.

Template variables: `$job` (Prometheus job label, default `pgxporter`) and `$instance` (one exporter).

### Coming soon

Native dashboards planned as follow-up work for collectors not covered by community dashboards:

- `pgxporter-io.json` — `pg_stat_io` per-backend-type I/O breakdown (PG 16+).
- `pgxporter-slru.json` — SLRU cache hit rates (PG 13+).
- `pgxporter-subscription.json` — once `pg_stat_subscription` collector ships (LIM-1043).
- `pgxporter-progress.json` — `pg_stat_progress_*` live operation progress.

## Verification note

`pgxporter-health.json` is hand-written JSON against the Grafana 10+ schema. It has been structurally validated but not end-to-end-rendered against a live Grafana by the project maintainer at ship-time. If a panel fails to import or render, file an issue with the Grafana version and the error — the likely fix is a schema-version bump in a single line. Patches welcome.
