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

Measured via `BenchmarkHeadToHead` in `exporter/headtohead_bench_test.go`. Both
exporters run as subprocesses against their own fresh PG 17.6 instances with
identical seed fixtures (50 tables × 100 rows), scraped over HTTP with a
10-iteration warmup. Running both as subprocesses is deliberate — see
[methodology note](#methodology-note-why-both-subprocesses) below.

Raw results: [`benchmarks/head-to-head/results/pg-17.6-defaults.txt`](benchmarks/head-to-head/results/pg-17.6-defaults.txt).

### 2026-04-20, PG 17.6, defaults-vs-defaults, local Linux/amd64 (AMD Ryzen AI 9 HX 370)

| Metric | pgxporter | postgres_exporter v0.19.1 | Delta |
| --- | --- | --- | --- |
| Scrape wall time (sec/op) | **8.5 ms** | 38.9 ms | pgxporter **4.6× faster** |
| Series emitted per scrape | **2,585** | 1,967 | pgxporter **+31% coverage** |
| Response body (bytes/scrape) | 274 KiB | 229 KiB | pgxporter +19% (more metrics) |
| Client-side B/op | 1.12 MiB | 629 KiB | pgxporter +82% (larger response to buffer) |
| Client-side allocs/op | 101 | 100 | essentially identical |

**Interpretation.** pgxporter wins decisively on the two numbers a Prometheus
operator feels directly: wall time (4.6× faster, driven by pgxpool connection
reuse + prepared-statement cache + errgroup-parallel collectors) and coverage
(31% more series from a richer default collector set). The client-side
allocation and memory numbers are near-identical — unsurprising, since both
sides of this benchmark run the same HTTP client code against responses of
comparable size.

### Methodology note: why both subprocesses

An earlier iteration of this benchmark ran pgxporter in-process via
`httptest.Server` and postgres_exporter as a subprocess. That produced
headline numbers that looked like pgxporter allocated 462× more than
postgres_exporter — a false apples-to-oranges comparison, because Go's
benchmark memstats only see the test process's allocations, never a
subprocess's. pgxporter's internal emission allocations were counted;
postgres_exporter's equivalent internal allocations were invisible.

The current benchmark avoids that by running **both** exporters as
subprocesses (via `testutil.StartPgxporter` and
`testutil.StartPostgresExporter`, which use the minimal CLI at `cmd/main.go`
and the published v0.19.1 binary respectively). The `B/op` and `allocs/op`
columns therefore measure strictly client-side HTTP work — equal for both,
as expected. Wall time and response payload size are unaffected by process
boundaries and remain fully comparable.

Server-side allocation comparison is a separate question (tracked as
LIM-1080). Use `BenchmarkExporterCollect` for pgxporter's in-process
allocation profile; equivalent in-process profiling for postgres_exporter
requires running it with `-memprofile` support which it doesn't expose
natively.

### Reproduce

```sh
go test -tags integration -bench HeadToHead -benchtime=100x -count=5 -timeout=15m ./exporter/
```

Three binaries auto-download or compile on first run: PG 17.6 (~10 MB),
postgres_exporter v0.19.1 (~10 MB), and pgxporter's own `cmd/` binary
(compiled locally via `go build`). ~60 s on a warm cache.
