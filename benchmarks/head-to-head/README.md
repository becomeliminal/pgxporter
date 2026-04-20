# Head-to-head: pgxporter vs postgres_exporter

A reproducible HTTP-scrape benchmark comparing pgxporter against [`prometheus-community/postgres_exporter`](https://github.com/prometheus-community/postgres_exporter) on the same PostgreSQL version, with identical seed data, at their default configurations, and — critically — with both exporters running as subprocesses so measurement is symmetric.

## How to run

From the repo root:

```sh
go test -tags integration -bench HeadToHead -benchtime=100x -count=5 -timeout=15m \
  ./exporter/ > benchmarks/head-to-head/results/$(date -u +%Y%m%d-%H%M%S).txt
```

Needs:

- Linux (the Postgres harness pins Linux/amd64 pre-built binaries).
- `~/.cache/pgxporter/postgres/17.6/` (PG binary, ~10 MB) and `~/.cache/pgxporter/postgres-exporter/v0.19.1/` (postgres_exporter binary, ~10 MB) — both auto-downloaded on first run.
- About 60 seconds for a 5 × 100 run on a warm cache. First-run cold downloads take longer.

Compare runs with `benchstat`:

```sh
go install golang.org/x/perf/cmd/benchstat@latest

benchstat benchmarks/head-to-head/results/pg-17.6-defaults.txt
```

## What's measured

For each sub-benchmark:

- **`sec/op`** — full HTTP `GET /metrics` wall time, client-observed.
- **`bytes/scrape`** — response body size in bytes.
- **`series/scrape`** — distinct Prometheus time series in the response (non-comment lines).
- **`B/op` + `allocs/op`** — memory and allocation count on the **client side** of the HTTP request. These numbers measure the benchmark's own HTTP client and response-parsing work, not either exporter's internal scrape machinery. They're essentially identical for both exporters by design (same client code running against responses of similar size).

## Methodology

Five choices that matter:

1. **Both exporters as subprocesses.** pgxporter runs via its `cmd/` binary (compiled by `testutil.StartPgxporter` on first use); postgres_exporter runs via its published v0.19.1 Linux binary. Running pgxporter in-process via `httptest.Server` would be simpler but would produce a false asymmetric comparison — Go's benchmark memstats only observe the test process, so pgxporter's internal allocations would be visible while postgres_exporter's would not. Both as subprocesses, measured strictly from the client side, is the only way to compare fairly.
2. **Fresh PG per sub-benchmark.** Each exporter gets its own `testutil.StartPG(b, "17.6")` instance with identical seed fixtures. Sharing one PG would unfairly advantage whichever exporter ran second (the first would churn pg_stat caches).
3. **Defaults vs defaults.** pgxporter ships 23 enabled-by-default collectors; postgres_exporter ships 10. We benchmark both at their defaults because that's what users actually deploy. A matched-subset comparison is possible but scoped out of the v1.0 baseline.
4. **HTTP scrape, not `Collect()` internal.** Hitting `/metrics` with a standard `http.Client` measures the full user-experienced path — serialization, HTTP transport, response parsing — all included.
5. **10-scrape warmup before `b.ResetTimer()`.** Lets prepared-statement caches, connection pools, and PG's shared_buffers settle into steady state. Cold-start numbers favour neither side meaningfully and add noise.

## Seed fixtures

Before either benchmark runs, `seedFixtures` creates 50 tables (`htoh_t000` … `htoh_t049`), each with 100 rows and an `ANALYZE` afterwards. This lifts `pg_stat_user_tables`, `pg_stat_user_indexes`, `pg_statio_user_tables`, and related views out of the "empty PG" regime where we'd be benchmarking PostgreSQL's overhead for zero-row SELECTs instead of realistic scrape work.

## Representative results (2026-04-20, PG 17.6, local Linux/amd64, AMD Ryzen AI 9 HX 370)

See [`results/pg-17.6-defaults.txt`](results/pg-17.6-defaults.txt) for the raw benchmark output. Summary:

| Metric | pgxporter | postgres_exporter v0.19.1 | Delta |
| --- | --- | --- | --- |
| Scrape wall time | **8.5 ms** | 38.9 ms | pgxporter **4.6× faster** |
| Series emitted | **2,585** | 1,967 | pgxporter **+31% coverage** |
| Response body | 274 KiB | 229 KiB | pgxporter +19% (more metrics) |
| Client-side B/op | 1.12 MiB | 629 KiB | pgxporter +82% (larger response to buffer) |
| Client-side allocs/op | 101 | 100 | essentially identical |

**Reading the numbers.** pgxporter wins decisively on the two metrics that matter to a Prometheus operator: wall time (4.6×) and coverage (+31%). The client-side `B/op` / `allocs/op` columns are expected to be near-equal — they reflect the benchmark's own HTTP client, not the exporter internals. Exporter-internal allocation profiling is a separate question (tracked as LIM-1080); for pgxporter use `BenchmarkExporterCollect`, for postgres_exporter they'd need to be built with pprof support.

### Why pgxporter is faster

Three architectural reasons, all validated directly or indirectly by this benchmark:

1. **Persistent pgxpool vs per-scrape `sql.DB`.** postgres_exporter opens a new `sql.DB` with `MaxOpenConns=1` on every scrape and tears it down at the end — every scrape pays TCP + auth cost. pgxporter holds a connection pool for the process lifetime.
2. **Prepared-statement cache.** pgx v5's `StatementCacheMode: "prepare"` keeps parse trees across scrapes. postgres_exporter on `lib/pq`'s `sql.DB`-per-scrape can't.
3. **Parallel collectors via errgroup.** pgxporter scrapes all collectors concurrently; postgres_exporter runs them serially. With 23 default collectors, the wall-clock win scales with the slowest collector's tail latency.

## What this benchmark is not

- **Not a multi-DB scaling test.** One DB per exporter. pgxporter's errgroup fan-out advantage grows with DB count, but postgres_exporter's multi-DB story routes through deprecated `--auto-discover-databases` or beta `/probe`, either of which confounds the comparison. Deferred to v1.1.
- **Not a PG-version matrix.** PG 17.6 only. Other versions are possible by changing the `testutil.StartPG(b, "17.6")` string; ratios shouldn't move much.
- **Not an over-the-network benchmark.** Localhost only. Adding network latency shifts both sides linearly and doesn't change the story.
- **Not a matched-collector-subset comparison.** "Defaults vs defaults" is what users actually run.
- **Not a server-side allocation comparison.** See the B/op caveat above.

## Versions

| Component | Version |
| --- | --- |
| pgxporter | at branch HEAD, built via `go build ./cmd` |
| postgres_exporter | v0.19.1 (released 2026-02-25) |
| PostgreSQL | 17.6 (via `testutil.StartPG`) |
| Go | 1.25+ |
| CPU | varies — documented in each result file header |
