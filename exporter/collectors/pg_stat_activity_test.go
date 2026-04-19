package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatActivityCollector_Describe(t *testing.T) {
	c := NewPgStatActivityCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 2; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatActivityCollector_Emit(t *testing.T) {
	tests := []struct {
		name  string
		rows  []*model.PgStatActivity
		wantN int
	}{
		{
			name: "row with both count + duration emits 2 metrics",
			rows: []*model.PgStatActivity{
				{
					Database:      text("postgres"),
					DatName:       text("postgres"),
					State:         text("active"),
					Count:         int8v(3),
					MaxTxDuration: floatv(1.5),
				},
			},
			wantN: 2,
		},
		{
			name: "row with only count emits 1 metric (duration NULL)",
			rows: []*model.PgStatActivity{
				{
					Database:      text("postgres"),
					DatName:       text("postgres"),
					State:         text("idle"),
					Count:         int8v(12),
					MaxTxDuration: pgtype.Float8{},
				},
			},
			wantN: 1,
		},
		{
			name: "row with all-NULL counters emits nothing",
			rows: []*model.PgStatActivity{
				{
					Database:      text("postgres"),
					DatName:       text("postgres"),
					State:         text("active"),
					Count:         pgtype.Int8{},
					MaxTxDuration: pgtype.Float8{},
				},
			},
			wantN: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewPgStatActivityCollector(nil)
			ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(tc.rows, ch) })
			if got := len(ms); got != tc.wantN {
				t.Errorf("emit produced %d metrics, want %d", got, tc.wantN)
			}
		})
	}
}
