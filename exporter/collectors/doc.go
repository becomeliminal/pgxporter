// Package collectors implements the [Collector] interface for every
// built-in pg_stat_* / pg_statio_* / pg_* view the exporter scrapes,
// plus the declarative [CollectorSpec] runner for user-defined SQL.
//
// Collectors are constructed via [ResolveCollectors] (honouring
// enable/disable sets and per-collector defaults) and driven by
// [exporter.Exporter], which fans them out under a per-scrape deadline
// and threads the context into every DB call.
//
// Namespace control:
//
//   - [SetMetricPrefix] flips the top-level "pg_stat" prefix to "pg"
//     for drop-in postgres_exporter compatibility. Must be called before
//     any collector is constructed.
//   - [MetricPrefixPg] / [MetricPrefixPgStat] are the supported values;
//     any other string is allowed but isn't covered by community dashboards.
//
// Adding a new collector:
//
//  1. Add an entry to collectorRegistry in registry.go.
//  2. Implement [Collector] — Describe + Scrape(ctx, ch).
//  3. Add an integration test in exporter/db that exercises the SQL
//     against a real PG 13–18 server via testutil.StartPG.
//
// Prefer [CollectorSpec] + LoadSpecsFromFile for new collectors that
// are pure SQL — the declarative path skips the boilerplate and is
// covered by the same self-instrumentation as built-in collectors.
package collectors
