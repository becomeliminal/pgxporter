package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgReplicationSlotsCollector_Describe(t *testing.T) {
	c := NewPgReplicationSlotsCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 7; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgReplicationSlotsCollector_Emit_PhysicalSlotPG16(t *testing.T) {
	c := NewPgReplicationSlotsCollector(nil)
	stats := []*model.PgReplicationSlot{{
		Database:          text("postgres"),
		SlotName:          text("standby1"),
		SlotType:          text("physical"),
		Plugin:            text(""),
		DatName:           text(""),
		Active:            pgtype.Bool{Bool: true, Valid: true},
		Temporary:         pgtype.Bool{Bool: false, Valid: true},
		RestartLsnBytes:   floatv(100_000),
		ConfirmedFlushLsn: pgtype.Float8{},
		RetainedWalBytes:  floatv(4096),
		WalStatus:         text("reserved"),
		SafeWalSizeBytes:  int8v(536870912),
		Conflicting:       pgtype.Bool{Bool: false, Valid: true},
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	// 6 valid fields (skip confirmed_flush_lsn which is NULL for physical slots).
	if got, want := len(ms), 6; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgReplicationSlotsCollector_Emit_PG12Shape(t *testing.T) {
	// Pre-13 → wal_status/safe_wal_size NULL, pre-16 → conflicting NULL.
	c := NewPgReplicationSlotsCollector(nil)
	stats := []*model.PgReplicationSlot{{
		Database:        text("postgres"),
		SlotName:        text("old_slot"),
		SlotType:        text("physical"),
		Active:          pgtype.Bool{Bool: true, Valid: true},
		Temporary:       pgtype.Bool{Bool: false, Valid: true},
		RestartLsnBytes: floatv(100),
		// Everything else NULL.
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	// active + temporary + restart_lsn = 3
	if got, want := len(ms), 3; got != want {
		t.Errorf("emit produced %d metrics, want %d (PG 12 shape skips gated fields)", got, want)
	}
}
