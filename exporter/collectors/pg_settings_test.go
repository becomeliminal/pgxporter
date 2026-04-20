package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgSettingsCollector_Describe(t *testing.T) {
	c := NewPgSettingsCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 1; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgSettingsCollector_Emit(t *testing.T) {
	c := NewPgSettingsCollector(nil)
	stats := []*model.PgSetting{
		{
			Database: text("postgres"),
			Name:     text("max_connections"),
			Unit:     text(""),
			VarType:  text("integer"),
			Value:    pgtype.Float8{Float64: 100, Valid: true},
		},
		{
			Database: text("postgres"),
			Name:     text("ssl"),
			Unit:     text(""),
			VarType:  text("bool"),
			Value:    pgtype.Float8{Float64: 1, Valid: true},
		},
		{
			Database: text("postgres"),
			Name:     text("work_mem"),
			Unit:     text("kB"),
			VarType:  text("integer"),
			Value:    pgtype.Float8{Float64: 4096, Valid: true},
		},
	}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 3; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgSettingsCollector_Emit_SkipsInvalid(t *testing.T) {
	c := NewPgSettingsCollector(nil)
	// Name NULL or value NULL → skipped.
	stats := []*model.PgSetting{
		{Database: text("postgres"), VarType: text("integer"), Value: pgtype.Float8{Float64: 1, Valid: true}}, // name invalid
		{Database: text("postgres"), Name: text("foo"), VarType: text("integer")},                              // value invalid
	}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 0; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
