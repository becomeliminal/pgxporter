//go:build integration

// Benchmarks for the full exporter Collect() cycle against a real PG.
// These demonstrate the architectural advantages of the pgxporter design:
// connection reuse across scrapes, prepared-statement caching, and
// errgroup-parallelised collector execution.
//
// Run:  go test -tags integration -bench . -benchtime=10x ./exporter
//
// benchtime=10x is usually what you want — scrape cycles are O(100ms) to
// O(1s) so the default 1s would do zero or one iteration.
package exporter_test

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/testutil"
)

// BenchmarkExporterCollect measures one full Collect() cycle against a
// single PG 17.6 instance. This is the tightest proxy for real-world
// scrape cost — every enabled collector runs in parallel and emits to
// the channel. Connection reuse and prepared-statement caching both
// benefit from b.N iterations hitting the same pool.
func BenchmarkExporterCollect(b *testing.B) {
	pg := testutil.StartPG(b, "17.6")

	opts := exporter.Opts{
		DBOpts: []db.Opts{{
			Host:                  pg.Host,
			Port:                  pg.Port,
			User:                  "postgres",
			Database:              "postgres",
			ApplicationName:       "pgxporter-benchmark",
			ConnectTimeout:        10 * time.Second,
			PoolMaxConns:          4,
			PoolMinConns:          1,
			PoolHealthCheckPeriod: time.Minute,
			PoolMaxConnLifetime:   time.Hour,
			PoolMaxConnIdleTime:   30 * time.Minute,
			StatementCacheMode:    "prepare",
		}},
		CollectionTimeout: 30 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	exp, err := exporter.New(ctx, opts)
	cancel()
	if err != nil {
		b.Fatalf("exporter.New: %v", err)
	}

	// Warm: trigger one scrape so prepared statements get cached.
	drain(exp.Collect)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drain(exp.Collect)
	}
}

// drain runs collect and discards every metric — closely models what
// the Prometheus HTTP handler does on a real /metrics request.
func drain(collect func(chan<- prometheus.Metric)) {
	ch := make(chan prometheus.Metric, 4096)
	go func() {
		defer close(ch)
		collect(ch)
	}()
	for range ch {
	}
}
