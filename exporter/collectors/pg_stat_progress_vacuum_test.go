package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatProgressVacuumCollector_Describe(t *testing.T) {
	c := NewPgStatProgressVacuumCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 4; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatProgressVacuumCollector_Emit(t *testing.T) {
	c := NewPgStatProgressVacuumCollector(nil)
	stats := []*model.PgStatProgressVacuum{{
		Database:         text("postgres"),
		DatName:          text("postgres"),
		RelID:            text("16384"),
		Phase:            text("scanning heap"),
		HeapBlksTotal:    int8v(10000),
		HeapBlksScanned:  int8v(4500),
		HeapBlksVacuumed: int8v(4200),
		IndexVacuumCount: int8v(1),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 4; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatProgressVacuumCollector_Emit_NoActiveVacuum(t *testing.T) {
	c := NewPgStatProgressVacuumCollector(nil)
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(nil, ch) })
	if got := len(ms); got != 0 {
		t.Errorf("idle cluster (nil rows) emitted %d metrics, want 0", got)
	}
}

func TestPgStatProgressVacuumCollector_Emit_PartialNullSkipped(t *testing.T) {
	c := NewPgStatProgressVacuumCollector(nil)
	stats := []*model.PgStatProgressVacuum{{
		Database:         text("postgres"),
		DatName:          text("postgres"),
		RelID:            text("16384"),
		Phase:            text("initializing"),
		HeapBlksTotal:    pgtype.Int8{},
		HeapBlksScanned:  pgtype.Int8{},
		HeapBlksVacuumed: pgtype.Int8{},
		IndexVacuumCount: int8v(0),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 1; got != want {
		t.Errorf("emit produced %d metrics, want %d (only index_vacuum_count Valid)", got, want)
	}
}
