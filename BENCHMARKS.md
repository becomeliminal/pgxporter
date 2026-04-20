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

## Scaling: multi-DB fan-out (2026-04-20, PG 17.6)

`BenchmarkExporterCollectMultiDB` spins up N `pgxpool.Pool`s pointed at the same underlying PG and scrapes them all in one Collect() cycle:

```
BenchmarkExporterCollectMultiDB/DBs_1     20     2.04 ms/op
BenchmarkExporterCollectMultiDB/DBs_4     20     4.21 ms/op    (4× DBs →  2.1× time)
BenchmarkExporterCollectMultiDB/DBs_16    20    28.58 ms/op   (16× DBs → 14×  time)
```

Sub-linear through 4 DBs (errgroup fan-out hiding DB-side latency). Approaching linear at 16 because all scrapes bottleneck on the single underlying PG — that's real-world behaviour, not a benchmark artefact. A serial exporter would sit at exactly N × single-DB cost.

## Scaling: prepared-statement cache warm vs cold (2026-04-20, PG 17.6)

`BenchmarkExporterCollectColdVsWarm` compares `StatementCacheMode="prepare"` (our default, parse once and reuse) against `StatementCacheMode="describe"` (no caching, simulates postgres_exporter's `sql.DB`-per-scrape pattern):

```
warm_prepare_cache    50    1.81 ms/op
no_prepare_cache      50    3.20 ms/op    (1.77× slower without cache)
```

The per-scrape parse cost matters: at 30 s scrape intervals, the delta per day is ~4 extra seconds of work across the exporter's lifetime. Negligible for a single instance, meaningful when deploying exporters at fleet scale.

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
| Scrape wall time (sec/op) | **8.7 ms** | 20.2 ms | pgxporter **2.3× faster** |
| Series emitted per scrape | **2,610** | 1,967 | pgxporter **+33% coverage** |
| Response body (bytes/scrape) | 275 KiB | 229 KiB | pgxporter +20% (more metrics) |
| Client-side B/op | 1.12 MiB | 628 KiB | pgxporter +82% (larger response to buffer) |
| Client-side allocs/op | 101 | 100 | essentially identical |

**Interpretation.** pgxporter wins decisively on the two numbers a Prometheus
operator feels directly: wall time (driven by pgxpool connection reuse,
prepared-statement cache, and errgroup-parallel collectors) and coverage
(31% more series from a richer default collector set). The client-side
allocation and memory numbers are near-identical — unsurprising, since both
sides of this benchmark run the same HTTP client code against responses of
comparable size.

The wall-time ratio varies with ambient system load (the benchmark spawns two
subprocesses on the test host). Different runs on the same machine have
shown 2.5× to 4.5×; rerun locally for your own numbers before citing them
externally.

### In-process allocation profile

`BenchmarkExporterCollect` measures pgxporter's pure `Collect()` cycle
without HTTP or subprocess overhead — the most direct proxy for
emission-path cost:

```
BenchmarkExporterCollect-24    50    1.26 ms/op    207 KB/op    1,584 allocs/op
```

pgxporter emits ~2,600 series per scrape for ~1,600 allocations — roughly
0.6 allocations per emitted series, mostly in the pgx row-scanning and
pgtype boxing paths. An earlier emission pattern (pre-`*Vec` refactor) sat
at ~10,700 allocations / 530 KB per Collect — the migration to stateful
`*prometheus.CounterVec` / `*prometheus.GaugeVec` children (with delta-
tracking for cumulative PG counters, preserving counter type semantics)
reduced per-scrape allocations by ~85% and per-scrape bytes by ~60%.

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

Server-side allocation comparison is a separate question — use
`BenchmarkExporterCollect` for pgxporter's in-process allocation
profile. Equivalent in-process profiling for postgres_exporter requires
running it with `-memprofile` support which it doesn't expose natively.

### Reproduce

```sh
go test -tags integration -bench HeadToHead -benchtime=100x -count=5 -timeout=15m ./exporter/
```

Three binaries auto-download or compile on first run: PG 17.6 (~10 MB),
postgres_exporter v0.19.1 (~10 MB), and pgxporter's own `cmd/` binary
(compiled locally via `go build`). ~60 s on a warm cache.
