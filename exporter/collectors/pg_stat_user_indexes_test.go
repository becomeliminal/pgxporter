package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatUserIndexesCollector_Describe(t *testing.T) {
	c := NewPgStatUserIndexesCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 3; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatUserIndexesCollector_Emit_NullCountersSkipped(t *testing.T) {
	c := NewPgStatUserIndexesCollector(nil)

	rows := []*model.PgStatUserIndex{
		// Fully populated row: 3 metrics.
		{
			Database:      text("postgres"),
			SchemaName:    text("public"),
			RelName:       text("orders"),
			IndexRelName:  text("orders_pkey"),
			IndexScan:     int8v(100),
			IndexTupRead:  int8v(500),
			IndexTupFetch: int8v(400),
		},
		// All counters NULL — the partitioned-parent shape that would crash
		// scans on plain int types. 0 metrics from this row.
		{
			Database:      text("postgres"),
			SchemaName:    text("public"),
			RelName:       text("orders_partitioned"),
			IndexRelName:  text("orders_partitioned_pkey"),
			IndexScan:     pgtype.Int8{},
			IndexTupRead:  pgtype.Int8{},
			IndexTupFetch: pgtype.Int8{},
		},
	}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(rows, ch) })
	if got, want := len(ms), 3; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
