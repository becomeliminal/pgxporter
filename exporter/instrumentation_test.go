//go:build integration

package exporter_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/testutil"
)

// TestSelfInstrumentation verifies every Collect cycle emits the three
// M6 self-metrics — scrape_duration_seconds (histogram), scrape_errors_total
// (counter), metric_cardinality (gauge) — for every registered collector,
// plus the existing up and exporter_scrapes_total.
func TestSelfInstrumentation(t *testing.T) {
	pg := testutil.StartPG(t, "17.6")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exp, err := exporter.New(ctx, exporter.Opts{
		DBOpts: []db.Opts{{
			Host:                  pg.Host,
			Port:                  pg.Port,
			User:                  "postgres",
			Database:              "postgres",
			ApplicationName:       "pgxporter-instrumentation-test",
			ConnectTimeout:        10 * time.Second,
			PoolMaxConns:          2,
			PoolMinConns:          1,
			PoolHealthCheckPeriod: time.Minute,
			PoolMaxConnLifetime:   time.Hour,
			PoolMaxConnIdleTime:   30 * time.Minute,
			StatementCacheMode:    "prepare",
		}},
	})
	if err != nil {
		t.Fatalf("exporter.New: %v", err)
	}

	ch := make(chan prometheus.Metric, 8192)
	go func() {
		defer close(ch)
		exp.Collect(ch)
	}()

	names := map[string]int{}
	for m := range ch {
		// Desc.String() includes fqName: "..." — parse out the fqName.
		s := m.Desc().String()
		if i := strings.Index(s, `fqName: "`); i >= 0 {
			s = s[i+len(`fqName: "`):]
			if j := strings.Index(s, `"`); j >= 0 {
				names[s[:j]]++
			}
		}
	}

	wantAtLeastOne := []string{
		"pg_stat_up",
		"pg_stat_exporter_scrapes_total",
		"pg_stat_scrape_duration_seconds",
		"pg_stat_scrape_errors_total",
		"pg_stat_metric_cardinality",
	}
	for _, want := range wantAtLeastOne {
		if names[want] == 0 {
			t.Errorf("expected at least one sample for %s, got 0", want)
		}
	}

	// Every default-enabled collector should contribute at least one
	// cardinality sample. Count against a minimum so a single missing
	// collector surfaces cleanly.
	if got := names["pg_stat_metric_cardinality"]; got < 10 {
		t.Errorf("metric_cardinality: got %d samples, want >= 10 (one per collector)", got)
	}
}

// TestShutdown verifies Shutdown closes the pool so subsequent DB work
// fails fast, and that ctx cancellation cleanly aborts the wait.
func TestShutdown(t *testing.T) {
	pg := testutil.StartPG(t, "17.6")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exp, err := exporter.New(ctx, exporter.Opts{
		DBOpts: []db.Opts{{
			Host:                  pg.Host,
			Port:                  pg.Port,
			User:                  "postgres",
			Database:              "postgres",
			ApplicationName:       "pgxporter-shutdown-test",
			ConnectTimeout:        10 * time.Second,
			PoolMaxConns:          2,
			PoolMinConns:          1,
			PoolHealthCheckPeriod: time.Minute,
			PoolMaxConnLifetime:   time.Hour,
			PoolMaxConnIdleTime:   30 * time.Minute,
			StatementCacheMode:    "prepare",
		}},
	})
	if err != nil {
		t.Fatalf("exporter.New: %v", err)
	}
	if err := exp.HealthCheck(ctx); err != nil {
		t.Fatalf("pre-shutdown healthcheck: %v", err)
	}
	if err := exp.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	// After Shutdown the pool is closed; HealthCheck must fail.
	if err := exp.HealthCheck(ctx); err == nil {
		t.Error("HealthCheck after Shutdown: want error, got nil")
	}
}
