package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

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
		{Database: text("postgres"), Name: text("max_connections"), Unit: text(""), VarType: text("integer"), Value: floatv(100)},
		{Database: text("postgres"), Name: text("shared_buffers"), Unit: text("8kB"), VarType: text("integer"), Value: floatv(16384)},
		{Database: text("postgres"), Name: text("fsync"), Unit: text(""), VarType: text("bool"), Value: floatv(1)},
	}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 3; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgSettingsCollector_Emit_SkipsNullValue(t *testing.T) {
	c := NewPgSettingsCollector(nil)
	stats := []*model.PgSetting{{
		Database: text("postgres"), Name: text("ignored"), VarType: text("integer"),
		Value: pgtype.Float8{},
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got := len(ms); got != 0 {
		t.Errorf("NULL value should be skipped, got %d metrics", got)
	}
}
