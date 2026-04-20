# Benchmarks

pgxporter is designed to be fast. This file documents how to run the bench
suite and captures representative numbers for comparison against future
changes.

## Running

The benchmarks live under `//go:build integration` because they need a real
Postgres to talk to. They spin up a PG 17.6 via the `becomeliminal/postgres`
plz plugin — same harness as the integration tests.

```sh
go test -tags integration -bench . -benchtime=10x -run '^$' ./exporter/
```

`-benchtime=10x` is usually what you want: scrape cycles are O(1–10ms) so the
default `-benchtime=1s` will loop thousands of times and produce a useful
distribution but also take a minute to run. `10x` is enough to confirm the
number hasn't regressed.

## Baseline (2026-04-20, PG 17.6, local Linux/amd64)

```
BenchmarkExporterCollect-N    300    ~4.6 ms/op    (all 24 collectors, single DB)
```

Interpretation: one full Prometheus `/metrics` scrape — 24 collectors
fanning out via errgroup, prepared-statement cache warm, connection reuse
across iterations — costs roughly **5 ms wall time** against a quiet local
PG. That's the ceiling on pgxporter overhead; real deployments add the
network round-trip, PG's view-materialisation cost for busy instances, and
whatever per-collector load the user has configured.

## Architectural advantages this demonstrates

1. **Connection reuse across scrapes.** The exporter holds a
   `pgxpool.Pool` for the whole process lifetime. `b.ResetTimer()`
   happens after one warm-up scrape, so every benchmarked `Collect()`
   reuses existing pool connections. `prometheus-community/postgres_exporter`
   opens a fresh `sql.DB` with `MaxOpenConns=1` per scrape and tears it
   down at the end — every scrape pays TLS handshake + auth cost.

2. **Prepared-statement cache.** `StatementCacheMode: "prepare"` makes
   pgx v5 hold onto parse trees for the duration of the pool. After the
   warm-up scrape, every SELECT is "EXECUTE <stmt>" with zero parse
   cost. Again, something `postgres_exporter` can't do with `lib/pq`'s
   `sql.DB`-per-scrape pattern.

3. **errgroup-parallel collectors.** Every registered collector runs
   concurrently inside `Exporter.Collect()`. 24 collectors vs. 24
   sequential queries is a wall-clock win proportional to the slowest
   collector's tail latency.

## Head-to-head vs postgres_exporter

Measured via `BenchmarkHeadToHead` in `exporter/headtohead_bench_test.go` — each
exporter runs against its own fresh PG 17.6 with identical seed fixtures (50
tables × 100 rows), scraped over HTTP with a 10-iteration warmup. See
[`benchmarks/head-to-head/README.md`](benchmarks/head-to-head/README.md) for
methodology.

Raw results: [`benchmarks/head-to-head/results/pg-17.6-defaults.txt`](benchmarks/head-to-head/results/pg-17.6-defaults.txt).

### 2026-04-20, PG 17.6, defaults-vs-defaults, local Linux/amd64 (AMD Ryzen AI 9 HX 370)

| Metric | pgxporter | postgres_exporter v0.19.1 | Delta |
| --- | --- | --- | --- |
| Scrape wall time (sec/op) | **14.6 ms** | 41.1 ms | pgxporter 2.8× faster |
| Series emitted per scrape | **2,581** | 1,967 | pgxporter +31% coverage |
| Response body (bytes/scrape) | 273 KiB | 229 KiB | pgxporter +19% (more metrics) |
| Memory per scrape (B/op) | 3.78 MiB | **627 KiB** | postgres_exporter 6× leaner |
| Allocations per scrape | 44,800 | **97** | postgres_exporter 462× fewer |

**Interpretation.** pgxporter wins on wall time and coverage — the two numbers a
Prometheus operator measures directly. postgres_exporter wins on allocation
count, which matters for memory pressure but not at realistic scrape
frequencies (15–30 s intervals turn 44K allocs/scrape into < 3 kallocs/sec).
The allocation gap is a real architectural difference — postgres_exporter
reuses buffers via `sync.Pool`, pgxporter's per-collector goroutine fan-out
allocates freely — and a potential future optimization.

To reproduce:

```sh
go test -tags integration -bench HeadToHead -benchtime=100x -count=5 -timeout=15m ./exporter/
```

Both binaries (PG + postgres_exporter) auto-download on first run. ~60 s on a
warm cache.
