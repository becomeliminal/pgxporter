package collectors

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatArchiverCollector_Describe(t *testing.T) {
	c := NewPgStatArchiverCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 5; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatArchiverCollector_Emit(t *testing.T) {
	c := NewPgStatArchiverCollector(nil)
	now := time.Unix(1_700_000_000, 0)
	stats := []*model.PgStatArchiver{{
		Database:         text("postgres"),
		ArchivedCount:    int8v(42),
		LastArchivedTime: pgtype.Timestamptz{Time: now, Valid: true},
		FailedCount:      int8v(1),
		LastFailedTime:   pgtype.Timestamptz{Time: now, Valid: true},
		StatsReset:       pgtype.Timestamptz{Time: now, Valid: true},
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 5; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatArchiverCollector_Emit_NoArchivingConfigured(t *testing.T) {
	// On a fresh PG with no archiving, counts are 0 (Valid=true) and the
	// last_*_time / stats_reset fields are NULL. We should still emit the
	// two counters plus nothing for the NULL timestamps.
	c := NewPgStatArchiverCollector(nil)
	stats := []*model.PgStatArchiver{{
		Database:      text("postgres"),
		ArchivedCount: int8v(0),
		FailedCount:   int8v(0),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 2; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
