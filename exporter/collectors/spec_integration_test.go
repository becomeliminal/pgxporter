//go:build integration

package collectors

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/testutil"
)

// TestSpecCollector_DatabaseSize exercises the declarative runner against
// a real PG by reimplementing pg_database_size as a CollectorSpec. The
// hand-written version is in pg_database_size.go; this just proves the
// spec mechanism is capable of the same thing.
func TestSpecCollector_DatabaseSize(t *testing.T) {
	pg := testutil.StartPG(t, "17.6")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := db.New(ctx, db.Opts{
		Host:                  pg.Host,
		Port:                  pg.Port,
		User:                  "postgres",
		Database:              "postgres",
		ApplicationName:       "pgxporter-spec-test",
		ConnectTimeout:        10 * time.Second,
		PoolMaxConns:          1,
		PoolMinConns:          1,
		PoolHealthCheckPeriod: time.Minute,
		PoolMaxConnLifetime:   time.Hour,
		PoolMaxConnIdleTime:   30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { _ = client.CheckConnection(ctx) })

	spec := CollectorSpec{
		Namespace: "pg",
		Subsystem: "database",
		SQL: `SELECT
			current_database() AS database,
			datname,
			pg_database_size(datname) AS bytes
		FROM pg_database
		WHERE datallowconn AND NOT datistemplate`,
		Labels: []string{"datname"},
		Metrics: []MetricSpec{
			{Name: "spec_size_bytes", Help: "Disk space used by the database, bytes", Column: "bytes"},
		},
	}
	c := NewSpecCollector(spec, []*db.Client{client})

	ms := drainMetrics(func(ch chan<- prometheus.Metric) {
		if err := c.Scrape(ctx, ch); err != nil {
			t.Errorf("Scrape: %v", err)
		}
	})
	if len(ms) == 0 {
		t.Fatalf("expected at least one metric, got 0")
	}
}

// TestSpecCollector_VersionGate verifies MinPGVersion causes the scrape
// to emit nothing on older servers.
func TestSpecCollector_VersionGate(t *testing.T) {
	pg := testutil.StartPG(t, "13.22")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := db.New(ctx, db.Opts{
		Host:                  pg.Host,
		Port:                  pg.Port,
		User:                  "postgres",
		Database:              "postgres",
		ApplicationName:       "pgxporter-spec-test",
		ConnectTimeout:        10 * time.Second,
		PoolMaxConns:          1,
		PoolMinConns:          1,
		PoolHealthCheckPeriod: time.Minute,
		PoolMaxConnLifetime:   time.Hour,
		PoolMaxConnIdleTime:   30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}

	spec := CollectorSpec{
		Subsystem:    "gated",
		SQL:          "SELECT current_database() AS database, 1::bigint AS x",
		MinPGVersion: [2]int{99, 0}, // unreachable
		Metrics: []MetricSpec{
			{Name: "x", Help: "x", Column: "x"},
		},
	}
	c := NewSpecCollector(spec, []*db.Client{client})
	ms := drainMetrics(func(ch chan<- prometheus.Metric) {
		if err := c.Scrape(ctx, ch); err != nil {
			t.Errorf("Scrape: %v", err)
		}
	})
	if len(ms) != 0 {
		t.Errorf("expected 0 metrics (version-gated), got %d", len(ms))
	}
}
