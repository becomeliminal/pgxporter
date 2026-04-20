// Package exporter implements a pgx-native Prometheus exporter for
// PostgreSQL. It runs one [Exporter] per process that scrapes a set of
// PostgreSQL servers ([db.Opts]) and exposes metrics via any standard
// [prometheus.Collector] registry.
//
// Typical usage:
//
//	exp, err := exporter.New(ctx, exporter.Opts{DBOpts: []db.Opts{{...}}})
//	if err != nil { log.Fatal(err) }
//	exp.Register()
//	http.Handle("/metrics", promhttp.Handler())
//	http.ListenAndServe(":9187", nil)
//
// The exporter is intentionally framework-agnostic — it doesn't bundle
// an HTTP server. Pair with [prometheus/exporter-toolkit] for TLS,
// basic auth, and graceful shutdown; see README for the reference
// wiring.
//
// Key building blocks:
//
//   - [Opts] — configure per-DB connection options, collector set, per-scrape
//     timeout, and the metric prefix (use [collectors.MetricPrefixPg] for
//     postgres_exporter dashboard compatibility).
//   - [Exporter.Shutdown] — drain in-flight scrapes and close every pgxpool.
//   - [Exporter.ExtendFromYAMLFile] — register custom [collectors.CollectorSpec]s
//     without writing Go. Replaces postgres_exporter's deprecated queries.yaml.
package exporter
