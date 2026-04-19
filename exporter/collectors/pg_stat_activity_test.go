package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatActivityCollector_Describe(t *testing.T) {
	c := NewPgStatActivityCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 4; got != want {
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
			name: "fully-populated bucket emits all 4 metrics",
			rows: []*model.PgStatActivity{{
				Database:                text("postgres"),
				DatName:                 text("postgres"),
				State:                   text("active"),
				WaitEventType:           text("none"),
				BackendType:             text("client backend"),
				Count:                   int8v(3),
				MaxTxDurationSeconds:    floatv(1.5),
				MaxQueryDurationSeconds: floatv(0.9),
				MaxBackendAgeSeconds:    floatv(3600),
			}},
			wantN: 4,
		},
		{
			name: "count only (no active txs) emits count + 3 zero-durations",
			rows: []*model.PgStatActivity{{
				Database:                text("postgres"),
				DatName:                 text("postgres"),
				State:                   text("idle"),
				WaitEventType:           text("Client"),
				BackendType:             text("client backend"),
				Count:                   int8v(12),
				MaxTxDurationSeconds:    floatv(0),
				MaxQueryDurationSeconds: floatv(0),
				MaxBackendAgeSeconds:    floatv(600),
			}},
			wantN: 4,
		},
		{
			name: "all-NULL row emits nothing",
			rows: []*model.PgStatActivity{{
				Database:                text("postgres"),
				DatName:                 text("postgres"),
				State:                   text("active"),
				WaitEventType:           text("none"),
				BackendType:             text("client backend"),
				Count:                   pgtype.Int8{},
				MaxTxDurationSeconds:    pgtype.Float8{},
				MaxQueryDurationSeconds: pgtype.Float8{},
				MaxBackendAgeSeconds:    pgtype.Float8{},
			}},
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
