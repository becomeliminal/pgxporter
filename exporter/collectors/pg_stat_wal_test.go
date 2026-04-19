package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatWalCollector_Describe(t *testing.T) {
	c := NewPgStatWalCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 9; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatWalCollector_Emit_PG14Shape(t *testing.T) {
	c := NewPgStatWalCollector(nil)
	stats := []*model.PgStatWal{{
		Database:       text("postgres"),
		WalRecords:     int8v(1000),
		WalFpi:         int8v(50),
		WalBytes:       floatv(1_000_000),
		WalBuffersFull: int8v(3),
		WalWrite:       int8v(100),
		WalSync:        int8v(50),
		WalWriteTime:   floatv(200.0), // ms
		WalSyncTime:    floatv(400.0), // ms
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	// 8 valid fields, stats_reset NULL.
	if got, want := len(ms), 8; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatWalCollector_Emit_PG15Shape(t *testing.T) {
	// PG 15 dropped wal_write / wal_sync / wal_write_time / wal_sync_time.
	c := NewPgStatWalCollector(nil)
	stats := []*model.PgStatWal{{
		Database:       text("postgres"),
		WalRecords:     int8v(1000),
		WalFpi:         int8v(50),
		WalBytes:       floatv(1_000_000),
		WalBuffersFull: int8v(3),
		WalWrite:       pgtype.Int8{},
		WalSync:        pgtype.Int8{},
		WalWriteTime:   pgtype.Float8{},
		WalSyncTime:    pgtype.Float8{},
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	// 4 valid counters only.
	if got, want := len(ms), 4; got != want {
		t.Errorf("emit produced %d metrics, want %d (PG 15+ should skip NULL write/sync fields)", got, want)
	}
}

func TestPgStatWalCollector_Emit_EmptyOnOlderPG(t *testing.T) {
	c := NewPgStatWalCollector(nil)
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(nil, ch) })
	if got := len(ms); got != 0 {
		t.Errorf("pre-14 (nil rows) emitted %d metrics, want 0", got)
	}
}
