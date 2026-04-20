package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatStatementsCollector_Describe(t *testing.T) {
	c := NewPgStatStatementsCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 19; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatStatementsCollector_Emit_NullsSkipped(t *testing.T) {
	c := NewPgStatStatementsCollector(nil)
	rows := []*model.PgStatStatement{
		{
			Database: text("postgres"),
			RolName:  text("postgres"),
			DatName:  text("postgres"),
			QueryID:  int8v(12345),
			Query:    text("SELECT 1"),
			Calls:    int8v(100),
			// Everything else NULL — only Calls should emit.
		},
	}
	c.emit(rows)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 1; got != want {
		t.Errorf("sparse row emitted %d metrics, want %d (only Calls is valid)", got, want)
	}
}

func TestPgStatStatementsCollector_Emit_QueryIDNullEmptyLabel(t *testing.T) {
	c := NewPgStatStatementsCollector(nil)
	// QueryID NULL → queryID label should be empty string, not crash.
	rows := []*model.PgStatStatement{
		{
			Database: text("postgres"),
			RolName:  text("postgres"),
			DatName:  text("postgres"),
			QueryID:  pgtype.Int8{},
			Query:    text("SELECT 1"),
			Calls:    int8v(1),
		},
	}
	c.emit(rows)
	ms := drainMetrics(c.collectInto)
	if len(ms) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(ms))
	}
}
