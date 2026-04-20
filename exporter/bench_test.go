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
	"fmt"
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

// BenchmarkExporterCollectMultiDB quantifies the errgroup fan-out win.
// One process, N DBs (each its own pgxpool pointed at the same underlying
// PG — so the work is real, not cached), all collectors on all DBs.
// Sub-benchmarks report per-DB-count timing; wall time should grow
// sub-linearly with DB count because pgxporter scrapes DBs in parallel.
//
// Rule of thumb: with N DBs, a serial exporter like postgres_exporter
// spends ~N× single-DB scrape time. pgxporter spends closer to
// single-DB time + (N-1) × pgxpool acquire overhead.
func BenchmarkExporterCollectMultiDB(b *testing.B) {
	pg := testutil.StartPG(b, "17.6")

	for _, n := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("DBs_%d", n), func(b *testing.B) {
			dbOpts := make([]db.Opts, n)
			for i := 0; i < n; i++ {
				dbOpts[i] = db.Opts{
					Host:                  pg.Host,
					Port:                  pg.Port,
					User:                  "postgres",
					Database:              "postgres",
					ApplicationName:       fmt.Sprintf("pgxporter-multi-%d", i),
					ConnectTimeout:        10 * time.Second,
					PoolMaxConns:          2,
					PoolMinConns:          1,
					PoolHealthCheckPeriod: time.Minute,
					PoolMaxConnLifetime:   time.Hour,
					PoolMaxConnIdleTime:   30 * time.Minute,
					StatementCacheMode:    "prepare",
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			exp, err := exporter.New(ctx, exporter.Opts{
				DBOpts:            dbOpts,
				CollectionTimeout: 30 * time.Second,
			})
			cancel()
			if err != nil {
				b.Fatalf("exporter.New: %v", err)
			}

			// Warm every pool + prepared-statement cache.
			drain(exp.Collect)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				drain(exp.Collect)
			}
		})
	}
}

// BenchmarkExporterCollectColdVsWarm contrasts scrapes that reuse a
// warm prepared-statement cache against scrapes that reconnect each
// time. The warm case is our normal operating mode; the cold case
// simulates what postgres_exporter pays on every scrape (it opens a
// fresh sql.DB per scrape). This is the single clearest architectural
// win pgxporter's pgx+pgxpool foundation provides.
func BenchmarkExporterCollectColdVsWarm(b *testing.B) {
	pg := testutil.StartPG(b, "17.6")
	opts := func(statementCache string) exporter.Opts {
		return exporter.Opts{
			DBOpts: []db.Opts{{
				Host:                  pg.Host,
				Port:                  pg.Port,
				User:                  "postgres",
				Database:              "postgres",
				ApplicationName:       "pgxporter-coldvswarm",
				ConnectTimeout:        10 * time.Second,
				PoolMaxConns:          4,
				PoolMinConns:          1,
				PoolHealthCheckPeriod: time.Minute,
				PoolMaxConnLifetime:   time.Hour,
				PoolMaxConnIdleTime:   30 * time.Minute,
				StatementCacheMode:    statementCache,
			}},
			CollectionTimeout: 30 * time.Second,
		}
	}

	b.Run("warm_prepare_cache", func(b *testing.B) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		exp, err := exporter.New(ctx, opts("prepare"))
		cancel()
		if err != nil {
			b.Fatalf("exporter.New: %v", err)
		}
		drain(exp.Collect) // warmup
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			drain(exp.Collect)
		}
	})

	// StatementCacheMode="describe" uses the anonymous prepared statement
	// per query — no caching. Proxy for postgres_exporter's sql.DB-per-scrape
	// behaviour from pgxporter's vantage point: every query pays parse cost.
	b.Run("no_prepare_cache", func(b *testing.B) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		exp, err := exporter.New(ctx, opts("describe"))
		cancel()
		if err != nil {
			b.Fatalf("exporter.New: %v", err)
		}
		drain(exp.Collect) // warmup (describes the set once)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			drain(exp.Collect)
		}
	})
}
