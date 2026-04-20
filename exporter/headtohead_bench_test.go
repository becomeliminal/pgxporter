//go:build integration

// Head-to-head HTTP-scrape benchmark: pgxporter vs
// prometheus-community/postgres_exporter.
//
// Both exporters run as subprocesses against their own fresh PG 17.6
// instance with identical seed fixtures (50 tables × 100 rows). The
// subprocess boundary is deliberate — running pgxporter in-process via
// httptest.Server would include its server-side allocations in the
// Go benchmark's process-local memstats while postgres_exporter's
// allocations would be invisible, producing a false apples-to-oranges
// comparison. Both as subprocesses, measured strictly from the HTTP
// client's side, is the only way to compare symmetrically.
//
// What this benchmark measures, and does not:
//
//   - sec/op — full HTTP round-trip wall time. Apples-to-apples.
//   - bytes/scrape — response body size. Apples-to-apples.
//   - series/scrape — time series emitted. Apples-to-apples.
//   - B/op, allocs/op — client-side only. NOT a measure of exporter
//     internal allocation. Identical for both (both are doing the same
//     HTTP GET + read + line-count). In-process allocation profiling
//     for pgxporter lives in BenchmarkExporterCollect.
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
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/becomeliminal/pgxporter/testutil"
)

// TestMain redirects the real stderr FD to /dev/null so the pgxporter
// subprocess binary's slog output (if leaked) doesn't interleave with
// benchmark-result rows and break benchstat parsing.
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
// with identical fixtures so neither is penalised by the other's accumulated
// pg_stat state, and each exporter runs as a subprocess so allocation
// measurement is symmetric (client-side only for both).
func BenchmarkHeadToHead(b *testing.B) {
	b.Run("pgxporter", func(b *testing.B) {
		pg := testutil.StartPG(b, "17.6")
		seedFixtures(b, pg)
		dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=postgres sslmode=disable", pg.Port)
		pe := testutil.StartPgxporter(b, dsn)
		runScrapeBench(b, pe.URL+"/metrics")
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

	// Warm-up: prepared-statement cache, connection pool, PG's shared_buffers.
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
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 && line[0] != '#' {
			series++
		}
	}
	return n, series, nil
}

// seedFixtures creates htohSeedTables tables each with htohSeedRowsPer rows.
// Lifts every pg_stat_user_tables / pg_statio_user_tables / etc. row count
// out of "empty view" territory so benchmarks reflect meaningful per-row
// work, not SELECT-from-nothing cost.
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

	if _, err := conn.Exec(ctx, "ANALYZE"); err != nil {
		b.Fatalf("seed: analyze: %v", err)
	}
}
