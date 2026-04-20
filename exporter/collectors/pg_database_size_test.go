package collectors

import (
	"testing"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgDatabaseSizeCollector_Describe(t *testing.T) {
	c := NewPgDatabaseSizeCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 1; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgDatabaseSizeCollector_Emit(t *testing.T) {
	c := NewPgDatabaseSizeCollector(nil)
	stats := []*model.PgDatabaseSize{
		{Database: text("postgres"), DatName: text("postgres"), Bytes: int8v(8_000_000)},
		{Database: text("postgres"), DatName: text("app"), Bytes: int8v(42_000_000)},
	}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 2; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
