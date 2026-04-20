package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatIOCollector_Describe(t *testing.T) {
	c := NewPgStatIOCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 15; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatIOCollector_Emit_FullyPopulatedBucket(t *testing.T) {
	c := NewPgStatIOCollector(nil)
	stats := []*model.PgStatIO{{
		Database: text("postgres"), BackendType: text("client backend"),
		Object: text("relation"), Context: text("normal"),
		Reads: int8v(1000), ReadTime: floatv(500.0),
		Writes: int8v(50), WriteTime: floatv(100.0),
		Writebacks: int8v(5), WritebackTime: floatv(10.0),
		Extends: int8v(2), ExtendTime: floatv(4.0),
		OpBytes:   int8v(8192),
		Hits:      int8v(9000),
		Evictions: int8v(100), Reuses: int8v(10),
		Fsyncs: int8v(3), FsyncTime: floatv(6.0),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	// 14 valid fields, stats_reset NULL.
	if got, want := len(ms), 14; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatIOCollector_Emit_PartialBucket(t *testing.T) {
	// autovacuum worker doing only reads: writes/fsyncs NULL.
	c := NewPgStatIOCollector(nil)
	stats := []*model.PgStatIO{{
		Database: text("postgres"), BackendType: text("autovacuum worker"),
		Object: text("relation"), Context: text("vacuum"),
		Reads: int8v(500), ReadTime: floatv(250.0),
		Hits:      int8v(2000),
		Evictions: int8v(30),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	// 4 valid fields.
	if got, want := len(ms), 4; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatIOCollector_Emit_EmptyOnOlderPG(t *testing.T) {
	c := NewPgStatIOCollector(nil)
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(nil, ch) })
	if got := len(ms); got != 0 {
		t.Errorf("pre-16 (nil rows) emitted %d metrics, want 0", got)
	}
}

// Reference pgtype.Int8{} so the import is used in case tests reorder.
var _ = pgtype.Int8{}
