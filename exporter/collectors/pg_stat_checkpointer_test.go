package collectors

import (
	"testing"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatCheckpointerCollector_Describe(t *testing.T) {
	c := NewPgStatCheckpointerCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 9; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatCheckpointerCollector_Emit(t *testing.T) {
	c := NewPgStatCheckpointerCollector(nil)
	stats := []*model.PgStatCheckpointer{{
		Database:           text("postgres"),
		NumTimed:           int8v(10),
		NumRequested:       int8v(2),
		RestartpointsTimed: int8v(0),
		RestartpointsReq:   int8v(0),
		RestartpointsDone:  int8v(0),
		WriteTime:          floatv(5000.0), // ms
		SyncTime:           floatv(1000.0),
		BuffersWritten:     int8v(500),
	}}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 8; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatCheckpointerCollector_Emit_EmptyOnOlderPG(t *testing.T) {
	c := NewPgStatCheckpointerCollector(nil)
	c.emit(nil)
	ms := drainMetrics(c.collectInto)
	if got := len(ms); got != 0 {
		t.Errorf("pre-17 (nil rows) emitted %d metrics, want 0", got)
	}
}
