package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatUserTableCollector_Describe(t *testing.T) {
	c := NewPgStatUserTableCollector(nil)
	// 19 baseline descriptors; the LIM-1012 rebase will raise to 23.
	if got, min := len(drainDescs(c.Describe)), 19; got < min {
		t.Errorf("Describe emitted %d, want >= %d", got, min)
	}
}

// The partitioned-parent shape from PG 17 that triggered the original
// "cannot assign 0 1 into *int" bug — all counter fields NULL.
func TestPgStatUserTableCollector_Emit_PartitionedParent(t *testing.T) {
	c := NewPgStatUserTableCollector(nil)
	rows := []*model.PgStatUserTable{
		{
			Database:         text("postgres"),
			SchemaName:       text("public"),
			RelName:          text("orders_partitioned"),
			SeqScan:          pgtype.Int8{},
			SeqTupRead:       pgtype.Int8{},
			IndexScan:        pgtype.Int8{},
			IndexTupFetch:    pgtype.Int8{},
			NTupInsert:       pgtype.Int8{},
			NTupUpdate:       pgtype.Int8{},
			NTupDelete:       pgtype.Int8{},
			NTupHotUpdate:    pgtype.Int8{},
			NLiveTup:         pgtype.Int8{},
			NDeadTup:         pgtype.Int8{},
			NModSinceAnalyze: pgtype.Int8{},
			LastVacuum:       pgtype.Timestamptz{},
			LastAutoVacuum:   pgtype.Timestamptz{},
			LastAnalyze:      pgtype.Timestamptz{},
			LastAutoAnalyze:  pgtype.Timestamptz{},
			VacuumCount:      pgtype.Int8{},
			AutoVacuumCount:  pgtype.Int8{},
			AnalyzeCount:     pgtype.Int8{},
			AutoAnalyzeCount: pgtype.Int8{},
		},
	}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(rows, ch) })
	if got, want := len(ms), 0; got != want {
		t.Errorf("partitioned-parent row emitted %d metrics, want %d (all NULL must be skipped)", got, want)
	}
}

func TestPgStatUserTableCollector_Emit_PopulatedRow(t *testing.T) {
	c := NewPgStatUserTableCollector(nil)
	rows := []*model.PgStatUserTable{
		{
			Database:         text("postgres"),
			SchemaName:       text("public"),
			RelName:          text("orders"),
			SeqScan:          int8v(1),
			SeqTupRead:       int8v(2),
			IndexScan:        int8v(3),
			IndexTupFetch:    int8v(4),
			NTupInsert:       int8v(5),
			NTupUpdate:       int8v(6),
			NTupDelete:       int8v(7),
			NTupHotUpdate:    int8v(8),
			NLiveTup:         int8v(9),
			NDeadTup:         int8v(10),
			NModSinceAnalyze: int8v(11),
			VacuumCount:      int8v(12),
			AutoVacuumCount:  int8v(13),
			AnalyzeCount:     int8v(14),
			AutoAnalyzeCount: int8v(15),
			// Timestamps NULL — scrape emits them as invalid (skipped).
		},
	}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(rows, ch) })
	// 15 integer counters should emit.
	if got, min := len(ms), 15; got < min {
		t.Errorf("populated row emitted %d metrics, want >= %d", got, min)
	}
}
