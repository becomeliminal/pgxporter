# Head-to-head: pgxporter vs postgres_exporter

A reproducible HTTP-scrape benchmark comparing pgxporter against [`prometheus-community/postgres_exporter`](https://github.com/prometheus-community/postgres_exporter) on the same PostgreSQL version, with identical seed data, and at their default configurations. Numbers here feed the "Why pgxporter" positioning in the README and BENCHMARKS.md.

## How to run

From the repo root:

```sh
go test -tags integration -bench HeadToHead -benchtime=100x -count=5 -timeout=15m \
  ./exporter/ > benchmarks/head-to-head/results/$(date -u +%Y%m%d-%H%M%S).txt
```

Needs:

- Linux amd64 (testutil pins pre-built binaries).
- `~/.cache/pgxporter/postgres/17.6/` (PG binary) and `~/.cache/pgxporter/postgres-exporter/v0.19.1/` (postgres_exporter binary) — both auto-downloaded on first run (~10 MB each).
- About 60 seconds for a 5 × 100 run. Longer if this is a cold download.

Compare two runs with `benchstat`:

```sh
go install golang.org/x/perf/cmd/benchstat@latest

benchstat benchmarks/head-to-head/results/pg-17.6-defaults.txt
```

## What's measured

For each sub-benchmark:

- **`sec/op`** — wall time per full HTTP `GET /metrics`, including serialization.
- **`bytes/scrape`** — response body size in bytes.
- **`series/scrape`** — distinct Prometheus time series in the response (lines not starting with `#`).
- **`B/op` + `allocs/op`** — memory allocated and allocation count per scrape.

## Methodology

Four choices that matter:

1. **Fresh PG per sub-benchmark.** Each exporter runs against its own `testutil.StartPG(b, "17.6")` instance with identical seed fixtures. Sharing one PG would unfairly advantage whichever exporter ran second (the first would churn pg_stat caches).
2. **Defaults vs defaults.** pgxporter ships 23 enabled-by-default collectors; postgres_exporter ships 10. We benchmark both at their defaults because that's what users actually deploy. A matched-subset comparison is possible but scoped out of the v1.0 baseline — see "Non-goals" below.
3. **HTTP scrape, not `Collect()` internal.** The benchmark hits `/metrics` over HTTP with a standard `http.Client`. This includes Prometheus text-format serialization, which both exporters do differently.
4. **10-scrape warmup before `b.ResetTimer()`.** Lets prepared-statement caches, connection pools, and PG's shared_buffers settle into steady state. Cold-start numbers would favour neither side meaningfully and add noise.

## Seed fixtures

Before either benchmark runs, `seedFixtures` creates 50 tables (`htoh_t000` … `htoh_t049`), each with 100 rows and an `ANALYZE` afterwards. This lifts `pg_stat_user_tables`, `pg_stat_user_indexes`, `pg_statio_user_tables`, and related views out of the "empty PG" regime where we'd be benchmarking PostgreSQL's overhead for zero-row SELECTs instead of realistic scrape work.

## Representative results (2026-04-20, PG 17.6, local Linux/amd64)

See [`results/pg-17.6-defaults.txt`](results/pg-17.6-defaults.txt) for the raw benchmark output. Summary:

| Metric | pgxporter | postgres_exporter | Ratio |
| --- | --- | --- | --- |
| Scrape wall time | **14.6 ms** | 41.1 ms | pgxporter 2.8× faster |
| Series emitted | **2,581** | 1,967 | pgxporter +31% coverage |
| Response body | 273 KiB | 229 KiB | pgxporter +19% (more metrics) |
| Memory per scrape | 3.78 MiB | **627 KiB** | postgres_exporter 6× leaner |
| Allocations per scrape | 44,800 | **97** | postgres_exporter 462× fewer |

**Reading the numbers.** pgxporter wins on wall time and coverage — which is what a Prometheus user cares about during a scrape. postgres_exporter wins on allocations, which matters for memory pressure but only at very high scrape rates. At a typical 15–30 s scrape interval, 44,800 allocations per scrape is nothing (< 3 kallocs/sec) and 4 MB per scrape is trivial per-second GC pressure.

The allocation delta is a real architectural difference: postgres_exporter aggressively reuses buffers via `sync.Pool`, while pgxporter's per-collector goroutine fan-out allocates freely. A potential future optimization; not v1.0 blocking.

## What this benchmark is not

- **Not a multi-DB scaling test.** One DB per exporter, not N. pgxporter's errgroup fan-out advantage grows with DB count, but postgres_exporter's multi-DB story runs through deprecated `--auto-discover-databases` or beta `/probe`, either of which confounds the comparison. Deferred to v1.1.
- **Not a PG-version matrix.** PG 17.6 only. Other versions are possible — change the `testutil.StartPG(b, "17.6")` string — but the relative ratios shouldn't move much and PG 13–18 testing is already covered by the integration matrix.
- **Not an over-the-network benchmark.** Localhost only. Adding network latency is a linear additive for both sides and doesn't change the story.
- **Not a matched-collector-subset comparison.** "Defaults vs defaults" is what users actually run. Matching subsets is defensible methodology but undersells the pgxporter-ships-more-collectors story.

## Versions

| Component | Version |
| --- | --- |
| pgxporter | at branch HEAD |
| postgres_exporter | v0.19.1 (released 2026-02-25) |
| PostgreSQL | 17.6 (via `testutil.StartPG`) |
| Go | 1.25+ |
| CPU | varies — documented in each result file header |
