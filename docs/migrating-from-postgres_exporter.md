# Migrating from postgres_exporter

A practical switch-over guide for teams running [`prometheus-community/postgres_exporter`](https://github.com/prometheus-community/postgres_exporter) who want to move to pgxporter.

Budget: about an afternoon for a single instance, plus a week of parallel observation before cutting over. No dashboard rewrites, no alert rule rewrites — metric names match when you opt into the compat prefix.

## Conceptual differences

Before we get into mappings, a few framing points so the rest makes sense:

| | postgres_exporter | pgxporter |
| --- | --- | --- |
| **Shape** | Pre-built binary with CLI flags | Library, you compose a `main.go` |
| **Multi-DB** | `DATA_SOURCE_NAME` list, or `/probe` endpoint | One `db.Opts` per DB in a slice passed at construction |
| **Custom metrics** | Deprecated `queries.yaml` | Declarative `CollectorSpec` YAML + Go interface escape hatch |
| **Auth** | Password, TLS certs via DSN | Password, TLS certs via DSN, **plus** cloud IAM via `AuthProvider` |
| **PG versions** | 9.4 → 18 | 13 → 18 |

The "library vs binary" difference is the biggest. Most postgres_exporter flags become struct fields on `exporter.Opts` or `db.Opts` in your `main.go`. If you want a CLI experience, wire the struct fields to flags with your flag library of choice (`flag`, `pflag`, `kingpin`, `go-flags`).

## Before you start: requirements check

- PostgreSQL 13+. If you're on PG 9.4–12, stay on postgres_exporter — pgxporter does not support these versions.
- Go 1.25+ to build.
- A plan to parallel-run both exporters for a week. The metrics are identical with `--metric-prefix=pg`, so a single Grafana dashboard can show both side-by-side and make any regressions obvious.

## Flag mapping

Every meaningful postgres_exporter flag, and its pgxporter equivalent:

| postgres_exporter | pgxporter |
| --- | --- |
| `--web.listen-address=:9187` | `http.ListenAndServe(":9187", …)` — stock `net/http` |
| `--web.telemetry-path=/metrics` | `http.Handle("/metrics", promhttp.Handler())` |
| `--web.config.file=…` | `exporter-toolkit`'s `web.ListenAndServe` — same YAML; see [README TLS section](../README.md#serving-metrics-over-tls--with-basic-auth) |
| `--log.level=debug` | `slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))` |
| `--log.format=json` | `slog.NewJSONHandler(os.Stdout, nil)` |
| `--extend.query-path=queries.yaml` | `exp.ExtendFromYAMLFile("custom.yaml")` — different schema, see [CollectorSpec](#custom-collectors-queriesyaml-to-collectorspec) below |
| `--disable-default-metrics` | `exporter.Opts.EnabledCollectors: []string{…}` — explicit whitelist |
| `--disable-settings-metrics` | n/a today; `pg_settings` collector not shipped yet (tracked as [LIM-1045](https://linear.app/liminal-cash/issue/LIM-1045)) |
| `--auto-discover-databases` | No direct equivalent. Discover databases with your own code, build a `[]db.Opts`, pass to `exporter.Opts{DBOpts: …}`. We use Kubernetes service discovery for this at Liminal. |
| `--exclude-databases=postgres,template0` | Handled during your discovery step before populating `DBOpts` |
| `--constant-labels=env=prod,role=primary` | Pass to Prometheus via the registry or via your scrape config — not an exporter concern |
| `--collector.bgwriter` | `EnabledCollectors: []string{"bgwriter"}` (or default — it's on by default) |
| `--no-collector.statements` | `DisabledCollectors: []string{"statements"}` (note: statements is off by default) |
| `--web.max-requests=N` | Bound at the `http.Server` level via `http.Server{Handler: http.MaxBytesHandler(…)}` or a reverse proxy |
| `--collection-timeout=1m` | `exporter.Opts.CollectionTimeout: time.Minute` |

## Environment variable mapping

| postgres_exporter | pgxporter |
| --- | --- |
| `DATA_SOURCE_NAME=postgresql://user:pass@host:5432/db` | Parse into `db.Opts{Host, Port, User, Password, Database}` |
| `DATA_SOURCE_URI=host:5432/db` | `db.Opts{Host, Port, Database}` |
| `DATA_SOURCE_USER=user` | `db.Opts{User: "user"}` |
| `DATA_SOURCE_PASS=pass` | `db.Opts{Password: "pass"}` — or use `AuthProvider` for cloud IAM |
| `PG_EXPORTER_WEB_LISTEN_ADDRESS=:9187` | Your flag wiring (pgxporter has no env-based config of its own) |

pgxporter's `db.Opts` struct is tagged for [`go-flags`](https://github.com/jessevdk/go-flags) so if you use that library you get CLI + env binding for free. See the README's single-target example.

## Metric name compatibility

postgres_exporter metric names look like `pg_stat_database_xact_commit`. pgxporter defaults to `pg_stat_database_xact_commit_total` (the Prometheus-canonical `_total` suffix for counters) under the `pg_stat` prefix.

To get **exactly** the postgres_exporter names so your existing dashboards and alert rules keep working:

```go
import "github.com/becomeliminal/pgxporter/exporter/collectors"

exp, err := exporter.New(ctx, exporter.Opts{
	MetricPrefix: collectors.MetricPrefixPg,
	DBOpts:       []db.Opts{…},
})
```

What this does:

- `pg_stat_database_*` → `pg_database_*`
- `pg_stat_bgwriter_*` → `pg_bgwriter_*`
- ...for every collector using the pg_stat namespace.

`pg_statio_*` and raw `pg_*` collectors (`pg_replication_slots`, `pg_database_size`) are unaffected — those are already PG-native names.

### Known metric-name deltas

A handful of metrics differ between the two exporters even under `MetricPrefixPg`. These are where we chose to diverge for correctness or consistency:

| postgres_exporter | pgxporter | Why |
| --- | --- | --- |
| `pg_stat_database_tup_returned` (counter, no `_total`) | `pg_database_tup_returned_total` | Counter naming convention |
| `pg_up{}` (one series, no labels) | `pg_up{database="<Name>"}` if `Opts.Name` set | Multi-exporter disambiguation |
| `pg_exporter_last_scrape_duration_seconds` (gauge) | `pg_scrape_duration_seconds{collector=…}` (histogram) | Per-collector observability |

The counter-`_total` mismatches affect PromQL queries that hard-code the metric name without the suffix. `rate(pg_database_tup_returned_total[5m])` works; `pg_database_tup_returned` does not. Grafana dashboards using `rate()` or `increase()` usually handle both cases because the query is written once per counter; static dashboards with hard-coded names need a search-and-replace.

## Dashboard reuse

Community postgres_exporter Grafana dashboards work unmodified with `MetricPrefixPg` set. The pair we've verified:

- [postgres_exporter's reference dashboard (ID 9628)](https://grafana.com/grafana/dashboards/9628)
- PMM's Postgres dashboard (PMM is not OSS but its panels reference the same metric names)

If you hit a panel that breaks, it's almost always one of the deltas in the table above.

## queries.yaml → CollectorSpec

`queries.yaml` is deprecated in postgres_exporter but still widely used. The pgxporter equivalent is `CollectorSpec` YAML — same shape, different field names, no macros.

postgres_exporter `queries.yaml`:

```yaml
pg_postmaster:
  query: "SELECT pg_postmaster_start_time as start_time_seconds from pg_postmaster_start_time()"
  master: true
  metrics:
    - start_time_seconds:
        usage: "GAUGE"
        description: "Time at which postmaster started"
```

pgxporter `custom.yaml`:

```yaml
- subsystem: postmaster
  sql: |
    SELECT current_database() AS database,
           extract(epoch FROM pg_postmaster_start_time()) AS start_time_seconds
  metrics:
    - name: start_time_seconds
      help: "Time at which postmaster started (unix seconds)"
      type: gauge
      column: start_time_seconds
```

Key shape differences:

- **Required `database` label column.** Every SQL must project a `database` column so metrics carry the database label correctly in multi-DB setups.
- **`subsystem` instead of root-level key.** pgxporter composes the final metric name as `<namespace>_<subsystem>_<name>`.
- **`type: counter|gauge`** instead of `usage: COUNTER|GAUGE`. Default is gauge.
- **`column:` names the SQL column** explicitly — no naming convention between SQL and metric.
- **`min_pg_version: "14.0"`** replaces postgres_exporter's `min_version: 140000` integer encoding.
- **No `master: true`.** pgxporter runs every collector against every DB by default.

Load with:

```go
if err := exp.ExtendFromYAMLFile("/etc/pgxporter/custom.yaml"); err != nil {
	log.Fatal(err)
}
```

A larger example with two specs including version gating is [in the README](../README.md#yaml-the-queriesyaml-replacement).

## Gotchas

- **PG 9.4–12 not supported.** If you have a long-tail PG version, stay on postgres_exporter for those instances.
- **`pg_stat_activity` has a higher default cardinality.** We expose `wait_event_type`, `wait_event`, `backend_type`, and `application_name` as labels by default. postgres_exporter collapses these. If you have hundreds of distinct `application_name` values you may want to disable the activity collector or use a custom `CollectorSpec` that aggregates more aggressively.
- **`pg_locks` emits blocking-chain data.** The `locks` collector uses `pg_blocking_pids()` which is fine for typical cluster sizes but can be expensive on OLTP databases with pathological contention patterns. Disable with `DisabledCollectors: []string{"locks"}` if you see it showing up in your slow-query log.
- **Log format is slog, not logrus.** `--log.format=json` translates to `slog.NewJSONHandler`. Log field names may not match one-for-one in log-aggregation queries.
- **No `--constant-labels` equivalent.** Attach labels via your Prometheus scrape config's `relabel_configs`, not at the exporter.

## Rollout pattern

The safest migration runs both exporters in parallel for a week:

1. **Build your pgxporter binary.** Start from the README quickstart. Wire flag parsing to your preferred flag library.
2. **Deploy as a second container** in the same pod / VM as postgres_exporter. Port 9187 for postgres_exporter (keep it), 9188 for pgxporter.
3. **Add a second Prometheus scrape** targeting the pgxporter port. Tag with `exporter="pgxporter"` vs `exporter="postgres_exporter"` so you can compare in PromQL.
4. **Turn on `MetricPrefixPg`** so the metric names line up. Existing dashboards reading `pg_database_xact_commit_total` now show both exporters' data.
5. **Observe for a week.** Watch for deltas:
   - Total sample count (`count({__name__=~"pg_.*"})`) — should be comparable, expect ~5–10% variance from cardinality differences.
   - Critical gauge values (`pg_up`, `pg_database_size_bytes`, `pg_stat_replication_lag_bytes`) — identical or trending together.
   - Scrape duration (`pg_scrape_duration_seconds` vs `pg_exporter_last_scrape_duration_seconds`) — pgxporter should be noticeably faster on multi-DB setups.
6. **Cut over** by removing the postgres_exporter scrape target and shutting down the old container. Keep the binary installed for one more week in case you need to roll back.
7. **Decommission** postgres_exporter.

## FAQ

**Q: Can I use `queries.yaml` unmodified?**
No. You need to port to the `CollectorSpec` schema. The shapes are similar; most files translate in under an hour.

**Q: What about `pg_stat_user_functions`?**
Not shipped yet. Use a custom `CollectorSpec` to cover it in the meantime.

**Q: Do I need to change Prometheus scrape config?**
Only the target (host:port). `scrape_interval`, `scrape_timeout`, `metrics_path`, and `relabel_configs` all stay the same.

**Q: What about the `/probe` multi-target pattern?**
We don't implement it. Multi-DB scraping is caller-driven: pass every DB in the `DBOpts` slice at construction time. For dynamic service discovery, rebuild the exporter when the DB list changes (typical deployment pattern in Kubernetes).

**Q: Is the alert rule surface the same?**
Yes, under `MetricPrefixPg`. Alert rule files that work against postgres_exporter work against pgxporter.

## If you hit a snag

File an issue at [github.com/becomeliminal/pgxporter/issues](https://github.com/becomeliminal/pgxporter/issues) with the postgres_exporter metric name, the pgxporter equivalent (or "I can't find it"), and the exporter version you're moving from.
