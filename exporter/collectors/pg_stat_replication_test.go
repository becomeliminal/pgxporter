package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatReplicationCollector_Describe(t *testing.T) {
	c := NewPgStatReplicationCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 7; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatReplicationCollector_Emit(t *testing.T) {
	c := NewPgStatReplicationCollector(nil)
	stats := []*model.PgStatReplication{{
		Database:         text("postgres"),
		ApplicationName:  text("replica1"),
		ClientAddr:       text("10.0.0.2"),
		State:            text("streaming"),
		SyncState:        text("async"),
		SentLagBytes:     floatv(0),
		WriteLagBytes:    floatv(128),
		FlushLagBytes:    floatv(256),
		ReplayLagBytes:   floatv(512),
		WriteLagSeconds:  floatv(0.001),
		FlushLagSeconds:  floatv(0.002),
		ReplayLagSeconds: floatv(0.003),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 7; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatReplicationCollector_Emit_StandbyShape(t *testing.T) {
	// On a standby the lag-bytes columns come through NULL because the
	// CASE WHEN pg_is_in_recovery() short-circuits them. The lag-seconds
	// columns come through as Valid because EXTRACT(EPOCH FROM NULL)
	// returns NULL too — we test with both NULL to confirm skip behavior.
	c := NewPgStatReplicationCollector(nil)
	stats := []*model.PgStatReplication{{
		Database:        text("postgres"),
		ApplicationName: text("cascading-replica"),
		ClientAddr:      text("10.0.0.3"),
		State:           text("streaming"),
		SyncState:       text("async"),
		// All lag values NULL
		SentLagBytes:     pgtype.Float8{},
		WriteLagBytes:    pgtype.Float8{},
		FlushLagBytes:    pgtype.Float8{},
		ReplayLagBytes:   pgtype.Float8{},
		WriteLagSeconds:  pgtype.Float8{},
		FlushLagSeconds:  pgtype.Float8{},
		ReplayLagSeconds: pgtype.Float8{},
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got := len(ms); got != 0 {
		t.Errorf("emit produced %d metrics, want 0 (all NULL)", got)
	}
}
