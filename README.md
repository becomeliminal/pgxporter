# pgxporter

[![CI](https://github.com/becomeliminal/pgxporter/actions/workflows/ci.yaml/badge.svg)](https://github.com/becomeliminal/pgxporter/actions/workflows/ci.yaml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

A pgx-native Prometheus exporter library for PostgreSQL 13–18. Intended as an alternative to [`prometheus-community/postgres_exporter`](https://github.com/prometheus-community/postgres_exporter), which is still built on [`lib/pq`](https://github.com/lib/pq) — a driver that's been in maintenance mode since 2021.

We leverage [`pgx/v5`](https://github.com/jackc/pgx), which is low-level, fast, actively maintained, and exposes PostgreSQL-specific features that `database/sql` doesn't — binary protocol, per-connection type registry, and the `BeforeConnect` hook that makes first-class cloud IAM auth possible.

This is a **library** — you compose it into your own `main.go` rather than running a pre-built binary. See [Quickstart](#quickstart) below.

## Why pgxporter

- **pgx v5 foundation.** Binary protocol, prepared-statement cache, persistent `pgxpool` per database, parallel scrapes via `errgroup`. `postgres_exporter` opens a new `sql.DB` (with `MaxOpenConns=1`) every scrape and runs collectors serially.
- **Cloud IAM auth, first-class.** AWS RDS, GCP CloudSQL, Azure Database — short-lived tokens minted via pgx's [`BeforeConnect`](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#Config) hook. No DSN-rewriting wrappers.
- **Declarative collector extension.** Add metrics in YAML without writing Go — the supported replacement for `postgres_exporter`'s deprecated `queries.yaml`. Custom Go collectors are still available as an escape hatch.
- **Drop-in dashboard compatibility.** `Opts.MetricPrefix = MetricPrefixPg` flips metric names from `pg_stat_database_*` to `pg_database_*`, so community `postgres_exporter` Grafana dashboards work unchanged.
- **PG 13–18 tested.** Real-Postgres integration matrix in CI across PG 13.22, 14.19, 15.14, 16.10, 17.6, 18.0.

## Feature matrix

Honest comparison against `postgres_exporter` v0.19.x.

| | pgxporter | postgres_exporter |
| --- | --- | --- |
| **Driver** | pgx/v5 (actively maintained) | lib/pq (maintenance mode) |
| **Connection pooling** | `pgxpool` persistent per DB | `sql.DB` with `MaxOpenConns=1`, reopened per scrape |
| **Parallel scrapes** | `errgroup` fan-out | serial |
| **Scrape context propagation** | full | full |
| **Password auth** | ✅ | ✅ |
| **Cloud IAM (RDS/CloudSQL/Azure)** | ✅ via `BeforeConnect` | ❌ DSN-rewriting hack |
| **TLS client certs** | ✅ via pgx DSN | ✅ |
| **Cluster pg_stat_* collectors** | ✅ (24 total, see below) | ✅ (~22) |
| **`pg_stat_io` (PG 16+)** | ✅ | ❌ |
| **`pg_stat_slru` (PG 13+)** | ✅ | ❌ |
| **`pg_stat_progress_*` (all 6)** | ✅ | partial (vacuum + analyze) |
| **Declarative YAML collectors** | ✅ `ExtendFromYAMLFile` | ❌ (deprecated `queries.yaml`) |
| **Per-collector enable/disable** | ✅ | ✅ |
| **Dashboard-compat metric prefix** | ✅ `MetricPrefixPg` | N/A (native) |
| **TLS listener + basic auth** | via `exporter-toolkit` | via `exporter-toolkit` |
| **Graceful shutdown** | ✅ `Exporter.Shutdown(ctx)` | ✅ |
| **Self-observability metrics** | ✅ scrape duration / errors / cardinality per-collector | partial |
| **PG versions tested** | 13–18 | 9.4–18 |
| **License** | Apache-2.0 | Apache-2.0 |
| **Community Grafana dashboards** | reuse postgres_exporter's via `MetricPrefixPg` | native |
| **Official Docker image** | not yet | Docker Hub |

Where `postgres_exporter` wins today: PG 9.4–13 long-tail support (we require PG 13+), the mature community dashboard ecosystem, and first-party container images. Coming tickets close the image/dashboards gap.

## Requirements

- Go 1.25+ (pgx 5.9.x requires 1.25)
- PostgreSQL 13, 14, 15, 16, 17, or 18

## Quickstart

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
)

func main() {
	ctx := context.Background()

	exp, err := exporter.New(ctx, exporter.Opts{
		DBOpts: []db.Opts{{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			Database:        "postgres",
			ApplicationName: "pgxporter",
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	exp.Register()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":9187", nil))
}
```

```
$ curl -s localhost:9187/metrics | head
# HELP pg_stat_activity_backends Number of backends by state and wait event
# TYPE pg_stat_activity_backends gauge
pg_stat_activity_backends{...}
...
```

## Collectors

All default-on except `statements`. Disable one via `exporter.Opts.DisabledCollectors: []string{"locks"}`; whitelist an explicit set via `Opts.EnabledCollectors`.

| Name | PostgreSQL view | Default |
| --- | --- | --- |
| `activity` | `pg_stat_activity` | ✅ |
| `archiver` | `pg_stat_archiver` | ✅ |
| `bgwriter` | `pg_stat_bgwriter` | ✅ |
| `checkpointer` | `pg_stat_checkpointer` (PG 17+) | ✅ |
| `database` | `pg_stat_database` | ✅ |
| `database_size` | `pg_database_size()` | ✅ |
| `io` | `pg_stat_io` (PG 16+) | ✅ |
| `io_user_indexes` | `pg_statio_user_indexes` | ✅ |
| `io_user_tables` | `pg_statio_user_tables` | ✅ |
| `locks` | `pg_locks` (with blocking-chain detection) | ✅ |
| `progress_analyze` | `pg_stat_progress_analyze` | ✅ |
| `progress_basebackup` | `pg_stat_progress_basebackup` | ✅ |
| `progress_cluster` | `pg_stat_progress_cluster` | ✅ |
| `progress_copy` | `pg_stat_progress_copy` | ✅ |
| `progress_create_index` | `pg_stat_progress_create_index` | ✅ |
| `progress_vacuum` | `pg_stat_progress_vacuum` | ✅ |
| `replication` | `pg_stat_replication` (primary) | ✅ |
| `replication_slots` | `pg_replication_slots` | ✅ |
| `slru` | `pg_stat_slru` (PG 13+) | ✅ |
| `statements` | `pg_stat_statements` | ⬜ off |
| `user_indexes` | `pg_stat_user_indexes` | ✅ |
| `user_tables` | `pg_stat_user_tables` | ✅ |
| `wal` | `pg_stat_wal` (PG 14+) | ✅ |
| `wal_receiver` | `pg_stat_wal_receiver` (standby) | ✅ |

`statements` is off by default because `pg_stat_statements` queries are expensive on busy clusters and the metric cardinality is high. Opt in explicitly.

## Configuration

`exporter.Opts` highlights (see [`pkg.go.dev`](https://pkg.go.dev/github.com/becomeliminal/pgxporter/exporter) for the full reference):

| Field | Purpose |
| --- | --- |
| `DBOpts` | One `db.Opts` per PostgreSQL server (required, ≥1) |
| `Name` | Const label `database=<Name>` on self-metrics when multiple exporters register in one Prometheus instance |
| `CollectionTimeout` | Per-scrape deadline. `0` → 1 min default; `-1` → no timeout |
| `MetricPrefix` | `collectors.MetricPrefixPg` for postgres_exporter-compat names |
| `EnabledCollectors` | Whitelist (empty = use defaults) |
| `DisabledCollectors` | Subtracts from the resolved set (wins if in both) |

`db.Opts` covers connection basics (`Host`/`Port`/`User`/`Password`/`Database`/`ApplicationName`), statement/lock/transaction timeouts, pgxpool tuning (`PoolMaxConns`/`PoolMaxConnLifetime`/etc.), pgx statement-cache mode, and the optional `AuthProvider` hook. ~20 fields total; see [`pkg.go.dev/.../exporter/db`](https://pkg.go.dev/github.com/becomeliminal/pgxporter/exporter/db).

## Custom collectors

### YAML (the `queries.yaml` replacement)

```yaml
- subsystem: custom_rates
  sql: |
    SELECT current_database() AS database, key, count(*) AS c
    FROM business.rate_events
    GROUP BY key
  labels: [key]
  metrics:
    - name: events_total
      help: Events by rate key
      type: counter
      column: c

- subsystem: bloat
  min_pg_version: "14.0"
  sql: SELECT current_database() AS database, schemaname, bytes FROM monitoring.bloat
  labels: [schemaname]
  metrics:
    - name: bytes
      help: Table bloat in bytes
      column: bytes
```

Load:

```go
if err := exp.ExtendFromYAMLFile("/etc/pgxporter/custom.yaml"); err != nil {
	log.Fatal(err)
}
```

`min_pg_version` accepts `"14.0"`, `[14, 0]`, or bare `"14"`. Omit `type` to emit a gauge. Every SQL must project a `database` label column so metrics are correctly labelled across multi-DB exporters.

### Go interface

For logic that outgrows YAML — custom type conversions, cross-row aggregation, conditional emission — implement the Go interface directly:

```go
type Collector interface {
	Describe(ch chan<- *prometheus.Desc)
	Scrape(ctx context.Context, ch chan<- prometheus.Metric) error
}
```

Register:

```go
exp := exporter.MustNew(ctx, opts).WithCustomCollectors(myCollector{}, anotherCollector{})
```

Implementations **must** thread `ctx` into every DB call — a pathological query that doesn't honour cancellation can hang the whole scrape cycle.

## Cloud IAM auth (RDS / CloudSQL / Azure)

Set `db.Opts.AuthProvider` and pgx will mint a fresh token for every new pool connection via its `BeforeConnect` hook. Tokens are rotated implicitly whenever pgxpool opens a new connection, so set `PoolMaxConnLifetime` shorter than the token's validity window.

AWS RDS example:

```go
import (
	"time"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/auth/awsrds"
)

provider, err := awsrds.NewDefault(ctx, "us-east-1")
if err != nil {
	log.Fatal(err)
}

dbOpts := db.Opts{
	Host:                "mydb.cluster-xyz.us-east-1.rds.amazonaws.com",
	Port:                5432,
	User:                "app_iam_user",
	Database:            "appdb",
	ApplicationName:     "pgxporter",
	AuthProvider:        provider,
	PoolMaxConnLifetime: 14 * time.Minute, // RDS IAM tokens are valid 15m; rotate at 14m
}
```

`awsrds.NewDefault` chains the standard AWS credential sources (env vars → shared config → IRSA → IMDS). Token minting is pure local SigV4 signing — no network round-trip per connection.

GCP CloudSQL and Azure Database providers follow the same pattern (implement `db.AuthProvider`):

```go
type AuthProvider interface {
	Password(ctx context.Context, host string, port int, user string) (string, error)
}
```

## postgres_exporter dashboard compatibility

`postgres_exporter` community Grafana dashboards assume names like `pg_database_xact_commit_total`. pgxporter defaults to `pg_stat_database_xact_commit_total`. Flip with `Opts.MetricPrefix`:

```go
import "github.com/becomeliminal/pgxporter/exporter/collectors"

exp, err := exporter.New(ctx, exporter.Opts{
	MetricPrefix: collectors.MetricPrefixPg,
	DBOpts:       []db.Opts{{...}},
})
```

Or set it before constructing any exporter (applies library-wide):

```go
collectors.SetMetricPrefix(collectors.MetricPrefixPg)
```

With this flag the community postgres_exporter dashboard set works unmodified.

## Serving metrics over TLS / with basic auth

pgxporter is a Prometheus `Collector` implementation — it stays framework-agnostic and doesn't bundle an HTTP server, so use any `net/http` listener you already run. For production setups matching `postgres_exporter`'s ergonomics, pair with [`prometheus/exporter-toolkit`](https://github.com/prometheus/exporter-toolkit), which provides a `--web.config.file` flag covering TLS, basic auth, and HTTP/2 out of the box:

```go
import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"

	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
)

func main() {
	webConfig := webflag.AddFlags(kingpin.CommandLine, ":9187")
	kingpin.Parse()

	exp := exporter.MustNew(context.Background(), exporter.Opts{DBOpts: []db.Opts{opts.DB}})
	exp.Register()

	http.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{}
	if err := web.ListenAndServe(srv, webConfig, slog.Default()); err != nil {
		log.Fatal(err)
	}
}
```

`--web.config.file` YAML (per [exporter-toolkit docs](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md)):

```yaml
tls_server_config:
  cert_file: /etc/pgxporter/tls.crt
  key_file: /etc/pgxporter/tls.key
basic_auth_users:
  prometheus: $2b$12$...  # bcrypt
```

## Graceful shutdown

```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer cancel()

// ... start exporter + HTTP listener ...

<-ctx.Done()

shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer shutdownCancel()

if err := srv.Shutdown(shutdownCtx); err != nil {
	log.Printf("http shutdown: %v", err)
}
if err := exp.Shutdown(shutdownCtx); err != nil {
	log.Printf("exporter shutdown: %v", err)
}
```

`Exporter.Shutdown` takes the scrape mutex so in-flight `Collect` calls finish before pools close. The per-scrape deadline from `Opts.CollectionTimeout` bounds the wait.

## Example deployments

### Single target

```go
import (
	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
)

var opts struct {
	DB db.Opts `group:"Postgres"`
}

func main() {
	flags.MustParse(&opts)
	exp := exporter.MustNew(context.Background(), exporter.Opts{DBOpts: []db.Opts{opts.DB}})
	exp.Register()
	// ...
}
```

Kubernetes:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # ...
spec:
  # ...
  template:
    spec:
      containers:
        - name: main
          image: <Go-Binary-Docker-Image>
          args:
            - --application_name=pgxporter
          resources:
            requests:
              memory: 20Mi
              cpu: 5m
            limits:
              memory: 40Mi
              cpu: 50m
          ports:
            - containerPort: 9187
              name: prometheus
          envFrom:
            - secretRef:
                name: your-db-secret
```

### Multi-target

Scrape N databases from one exporter process — the pattern used in production at Liminal across 18 Postgres instances.

```go
import (
	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
)

var opts struct {
	FirstDB  db.Opts `namespace:"first" env-namespace:"FIRST"`
	SecondDB db.Opts `namespace:"second" env-namespace:"SECOND"`
}

func main() {
	flags.MustParse(&opts)
	exp := exporter.MustNew(context.Background(), exporter.Opts{
		DBOpts: []db.Opts{opts.FirstDB, opts.SecondDB},
	})
	exp.Register()
	// ...
}
```

Kubernetes:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # ...
spec:
  # ...
  template:
    spec:
      containers:
        - name: main
          image: <Go-Binary-Docker-Image>
          args:
            - --first.application_name=pgxporter
            - --second.application_name=pgxporter
          ports:
            - containerPort: 9187
              name: prometheus
          envFrom:
            - secretRef:
                name: your-first-db-secret
              prefix: FIRST_
            - secretRef:
                name: your-second-db-secret
              prefix: SECOND_
```

## Exporter self-metrics

Emitted at the default `pg_stat` prefix (flip with `Opts.MetricPrefix`):

- `pg_stat_up` (gauge) — `1` if the last scrape succeeded, `0` on error.
- `pg_stat_exporter_scrapes_total` (counter) — cumulative scrape count.
- `pg_stat_scrape_duration_seconds{collector}` (histogram) — per-collector wall time.
- `pg_stat_scrape_errors_total{collector}` (counter) — per-collector error count. Pre-initialised to 0 for every resolved collector so `rate()` returns a clean zero on healthy instances.
- `pg_stat_metric_cardinality{collector}` (gauge) — metric count emitted by a collector on its last scrape. Early-warning signal for cardinality explosions.

All self-metrics carry a `database=<Opts.Name>` const label when `Opts.Name` is set.

## License

Apache-2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE). Relicensed from AGPL-3.0 in April 2026 to align with the Prometheus exporter ecosystem (`postgres_exporter`, `client_golang`, `exporter-toolkit` are all Apache-2.0) and unblock corporate adoption blocked by AGPL §13 network-distribution obligations.

## Benchmarks

See [BENCHMARKS.md](BENCHMARKS.md) for head-to-head scrape-duration numbers.
