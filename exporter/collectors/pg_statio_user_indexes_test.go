package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatIOUserIndexesCollector_Describe(t *testing.T) {
	c := NewPgStatIOUserIndexesCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 2; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatIOUserIndexesCollector_Emit(t *testing.T) {
	c := NewPgStatIOUserIndexesCollector(nil)
	rows := []*model.PgStatIOUserIndex{
		{
			Database:      text("postgres"),
			SchemaName:    text("public"),
			RelName:       text("a"),
			IndexRelName:  text("a_pk"),
			IndexBlksRead: int8v(42),
			IndexBlksHit:  int8v(100),
		},
		{
			Database:      text("postgres"),
			SchemaName:    text("public"),
			RelName:       text("b"),
			IndexRelName:  text("b_pk"),
			IndexBlksRead: pgtype.Int8{},
			IndexBlksHit:  pgtype.Int8{},
		},
	}
	c.emit(rows)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 2; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
