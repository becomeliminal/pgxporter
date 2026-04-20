package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatBgwriterCollector_Describe(t *testing.T) {
	c := NewPgStatBgwriterCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 11; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatBgwriterCollector_Emit_PG16Shape(t *testing.T) {
	c := NewPgStatBgwriterCollector(nil)
	stats := []*model.PgStatBgwriter{{
		Database:            text("postgres"),
		BuffersClean:        int8v(10),
		MaxwrittenClean:     int8v(1),
		BuffersAlloc:        int8v(100),
		CheckpointsTimed:    int8v(5),
		CheckpointsReq:      int8v(2),
		CheckpointWriteTime: floatv(1200.0), // ms
		CheckpointSyncTime:  floatv(200.0),
		BuffersCheckpoint:   int8v(50),
		BuffersBackend:      int8v(20),
		BuffersBackendFsync: int8v(0),
	}}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	// 10 valid fields (all but StatsReset).
	if got, want := len(ms), 10; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatBgwriterCollector_Emit_PG17Shape(t *testing.T) {
	c := NewPgStatBgwriterCollector(nil)
	// PG 17 leaves the checkpoint / backend columns NULL.
	stats := []*model.PgStatBgwriter{{
		Database:            text("postgres"),
		BuffersClean:        int8v(10),
		MaxwrittenClean:     int8v(1),
		BuffersAlloc:        int8v(100),
		CheckpointsTimed:    pgtype.Int8{},
		CheckpointsReq:      pgtype.Int8{},
		CheckpointWriteTime: pgtype.Float8{},
		CheckpointSyncTime:  pgtype.Float8{},
		BuffersCheckpoint:   pgtype.Int8{},
		BuffersBackend:      pgtype.Int8{},
		BuffersBackendFsync: pgtype.Int8{},
	}}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	// Only the 3 retained counters emit.
	if got, want := len(ms), 3; got != want {
		t.Errorf("emit produced %d metrics, want %d (PG 17 should skip NULL checkpoint/backend fields)", got, want)
	}
}
