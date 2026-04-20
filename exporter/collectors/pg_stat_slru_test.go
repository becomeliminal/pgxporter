package collectors

import (
	"testing"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatSLRUCollector_Describe(t *testing.T) {
	c := NewPgStatSLRUCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 8; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatSLRUCollector_Emit(t *testing.T) {
	c := NewPgStatSLRUCollector(nil)
	stats := []*model.PgStatSLRU{{
		Database:    text("postgres"),
		Name:        text("CommitTs"),
		BlksZeroed:  int8v(10),
		BlksHit:     int8v(1000),
		BlksRead:    int8v(30),
		BlksWritten: int8v(5),
		BlksExists:  int8v(0),
		Flushes:     int8v(2),
		Truncates:   int8v(1),
	}}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	// 7 counters valid, stats_reset NULL.
	if got, want := len(ms), 7; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatSLRUCollector_Emit_Empty(t *testing.T) {
	c := NewPgStatSLRUCollector(nil)
	c.emit(nil)
	ms := drainMetrics(c.collectInto)
	if got := len(ms); got != 0 {
		t.Errorf("empty rows emitted %d metrics, want 0", got)
	}
}
