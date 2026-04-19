package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatIOUserTableCollector_Describe(t *testing.T) {
	c := NewPgStatIOUserTableCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 8; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatIOUserTableCollector_Emit(t *testing.T) {
	c := NewPgStatIOUserTableCollector(nil)
	// One fully-populated row (8 metrics), one all-NULL (0 metrics).
	rows := []*model.PgStatIOUserTable{
		{
			Database:      text("postgres"),
			SchemaName:    text("public"),
			RelName:       text("a"),
			HeapBlksRead:  int8v(1),
			HeapBlksHit:   int8v(2),
			IndexBlksRead: int8v(3),
			IndexBlksHit:  int8v(4),
			ToastBlksRead: int8v(5),
			ToastBlksHit:  int8v(6),
			TidxBlksRead:  int8v(7),
			TidxBlksHit:   int8v(8),
		},
		{
			Database:      text("postgres"),
			SchemaName:    text("public"),
			RelName:       text("b_partitioned"),
			HeapBlksRead:  pgtype.Int8{},
			HeapBlksHit:   pgtype.Int8{},
			IndexBlksRead: pgtype.Int8{},
			IndexBlksHit:  pgtype.Int8{},
			ToastBlksRead: pgtype.Int8{},
			ToastBlksHit:  pgtype.Int8{},
			TidxBlksRead:  pgtype.Int8{},
			TidxBlksHit:   pgtype.Int8{},
		},
	}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(rows, ch) })
	if got, want := len(ms), 8; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
