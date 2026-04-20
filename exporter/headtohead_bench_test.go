//go:build integration

// Head-to-head HTTP-scrape benchmark: pgxporter vs
// prometheus-community/postgres_exporter.
//
// Each sub-benchmark runs against its own fresh PG 17.6 instance with
// identical seed fixtures (50 tables × 100 rows) — a shared PG would
// unfairly advantage the second runner by having warm pg_stat caches.
//
// Both exporters run their default collector sets. Scrapes go over HTTP
// (not Collect() internally) so we measure the full user-experienced
// path including serialization.
//
// Run:
//
//	go test -tags integration -bench HeadToHead -benchtime=100x -count=5 ./exporter/ > results.txt
//	benchstat results.txt
package exporter_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/testutil"
)

// TestMain redirects the real stderr FD to /dev/null so
// logging.NewLogger's already-captured os.Stderr reference writes nowhere.
// Without this, pgxporter's INFO-level slog lines interleave with
// benchmark-result rows and break benchstat parsing.
//
// syscall.Dup2 swaps FD 2 at the kernel level — fixes output even for
// handlers that captured os.Stderr at package-init time.
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
	if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		_ = syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))
	}
	os.Exit(m.Run())
}

const (
	htohSeedTables  = 50
	htohSeedRowsPer = 100
	htohWarmup      = 10
)

// BenchmarkHeadToHead compares one Prometheus HTTP scrape against pgxporter
// vs postgres_exporter. Each sub-benchmark gets its own fresh PG instance
// with identical fixtures so neither is penalised by the other's
// accumulated pg_stat state. That's slower to orchestrate than a single
// shared PG, but it's the only fair way to compare — the first exporter
// to run would otherwise churn pg_stat_statements / pg_stat_activity /
// pg_stat_database counters for the second.
func BenchmarkHeadToHead(b *testing.B) {
	b.Run("pgxporter", func(b *testing.B) {
		pg := testutil.StartPG(b, "17.6")
		seedFixtures(b, pg)
		srv := setupPgxporterHTTP(b, pg)
		defer srv.Close()
		runScrapeBench(b, srv.URL+"/metrics")
	})

	b.Run("postgres_exporter", func(b *testing.B) {
		pg := testutil.StartPG(b, "17.6")
		seedFixtures(b, pg)
		dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=postgres sslmode=disable", pg.Port)
		pe := testutil.StartPostgresExporter(b, dsn)
		runScrapeBench(b, pe.URL+"/metrics")
	})
}

// runScrapeBench does the shared HTTP-scrape loop for either exporter.
// Warms up htohWarmup times, resets the timer, hits /metrics b.N times,
// and reports the last response's body size + series count as custom metrics.
func runScrapeBench(b *testing.B, url string) {
	b.Helper()
	client := &http.Client{Timeout: 30 * time.Second}

	// Warm-up: prepared-statement cache for pgxporter, sql.DB + connection
	// for postgres_exporter, PG's shared_buffers for both.
	for i := 0; i < htohWarmup; i++ {
		if _, _, err := scrape(client, url); err != nil {
			b.Fatalf("warmup scrape %d: %v", i, err)
		}
	}

	var lastBytes, lastSeries int64

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, s, err := scrape(client, url)
		if err != nil {
			b.Fatalf("scrape %d: %v", i, err)
		}
		lastBytes = n
		lastSeries = s
	}
	b.StopTimer()

	b.ReportMetric(float64(lastBytes), "bytes/scrape")
	b.ReportMetric(float64(lastSeries), "series/scrape")
}

// scrape performs one GET /metrics, drains the body, and returns (bytes read,
// series count, error). Series count = lines in the Prometheus text-format
// response that don't start with '#'.
func scrape(client *http.Client, url string) (int64, int64, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return 0, 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	var body bytes.Buffer
	n, err := io.Copy(&body, resp.Body)
	if err != nil {
		return n, 0, err
	}

	var series int64
	scanner := bufio.NewScanner(bytes.NewReader(body.Bytes()))
	// Prom text format can have 100KB+ lines for some histograms; bump buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 && line[0] != '#' {
			series++
		}
	}
	return n, series, nil
}

// setupPgxporterHTTP builds a fresh pgxporter Exporter pointed at pg and
// wraps it in an httptest.Server serving /metrics via promhttp.
func setupPgxporterHTTP(b *testing.B, pg *testutil.PG) *httptest.Server {
	b.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exp, err := exporter.New(ctx, exporter.Opts{
		DBOpts: []db.Opts{{
			Host:                  pg.Host,
			Port:                  pg.Port,
			User:                  "postgres",
			Database:              "postgres",
			ApplicationName:       "pgxporter-headtohead",
			ConnectTimeout:        10 * time.Second,
			PoolMaxConns:          4,
			PoolMinConns:          1,
			PoolHealthCheckPeriod: time.Minute,
			PoolMaxConnLifetime:   time.Hour,
			PoolMaxConnIdleTime:   30 * time.Minute,
			StatementCacheMode:    "prepare",
		}},
		CollectionTimeout: 30 * time.Second,
	})
	if err != nil {
		b.Fatalf("exporter.New: %v", err)
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(exp); err != nil {
		b.Fatalf("reg.Register: %v", err)
	}

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		Timeout: 30 * time.Second,
	})
	return httptest.NewServer(handler)
}

// seedFixtures creates htohSeedTables tables each with htohSeedRowsPer rows.
// The goal: lift every pg_stat_user_tables / pg_statio_user_tables / etc. row
// count out of "empty view" territory so the benchmarks reflect meaningful
// per-row work, not SELECT-from-nothing cost.
func seedFixtures(b *testing.B, pg *testutil.PG) {
	b.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, pg.DSN)
	if err != nil {
		b.Fatalf("seed: connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	for i := 0; i < htohSeedTables; i++ {
		name := fmt.Sprintf("htoh_t%03d", i)
		if _, err := conn.Exec(ctx, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s (id int PRIMARY KEY, payload text NOT NULL)", name,
		)); err != nil {
			b.Fatalf("seed: create table %s: %v", name, err)
		}
		// Bulk insert — one statement with multi-row VALUES beats N single-row
		// inserts by ~10x wall time.
		var vals strings.Builder
		for r := 0; r < htohSeedRowsPer; r++ {
			if r > 0 {
				vals.WriteString(", ")
			}
			fmt.Fprintf(&vals, "(%d, 'payload-%d-%d')", r, i, r)
		}
		if _, err := conn.Exec(ctx, fmt.Sprintf(
			"INSERT INTO %s (id, payload) VALUES %s ON CONFLICT DO NOTHING", name, vals.String(),
		)); err != nil {
			b.Fatalf("seed: insert into %s: %v", name, err)
		}
	}

	// ANALYZE so pg_stat_user_tables' n_live_tup / last_analyze columns are
	// populated before we start benchmarking.
	if _, err := conn.Exec(ctx, "ANALYZE"); err != nil {
		b.Fatalf("seed: analyze: %v", err)
	}
}

