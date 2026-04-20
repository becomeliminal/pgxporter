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

Deferred to LIM-1053 — setting up a fair side-by-side benchmark needs
postgres_exporter compiled locally and pointed at the same PG instance,
plus matching collector sets so the comparison is apples-to-apples. The
numbers above are a starting point; the follow-up ticket publishes the
comparison for the README.
