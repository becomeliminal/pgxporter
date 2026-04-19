package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgLocksCollector_Describe(t *testing.T) {
	c := NewPgLocksCollector(nil)
	descs := drainDescs(c.Describe)
	if got, want := len(descs), 1; got != want {
		t.Fatalf("Describe emitted %d descriptors, want %d", got, want)
	}
}

func TestPgLocksCollector_Emit(t *testing.T) {
	tests := []struct {
		name  string
		rows  []*model.PgLock
		wantN int
	}{
		{
			name: "two valid rows emit two metrics",
			rows: []*model.PgLock{
				{
					Database: text("postgres"),
					DatName:  text("postgres"),
					Mode:     text("AccessShareLock"),
					Count:    int8v(5),
				},
				{
					Database: text("postgres"),
					DatName:  text("postgres"),
					Mode:     text("ExclusiveLock"),
					Count:    int8v(1),
				},
			},
			wantN: 2,
		},
		{
			name: "NULL count row is skipped",
			rows: []*model.PgLock{
				{
					Database: text("postgres"),
					DatName:  text("postgres"),
					Mode:     text("AccessShareLock"),
					Count:    pgtype.Int8{}, // Valid: false
				},
			},
			wantN: 0,
		},
		{
			name: "mixed valid and NULL — only valid emits",
			rows: []*model.PgLock{
				{Database: text("a"), DatName: text("a"), Mode: text("m"), Count: int8v(3)},
				{Database: text("b"), DatName: text("b"), Mode: text("m"), Count: pgtype.Int8{}},
			},
			wantN: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewPgLocksCollector(nil)
			metrics := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(tc.rows, ch) })
			if got := len(metrics); got != tc.wantN {
				t.Errorf("emit produced %d metrics, want %d", got, tc.wantN)
			}
		})
	}
}

// drainDescs runs fn with a buffered chan and returns everything it emits.
func drainDescs(fn func(chan<- *prometheus.Desc)) []*prometheus.Desc {
	ch := make(chan *prometheus.Desc, 64)
	go func() {
		fn(ch)
		close(ch)
	}()
	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}
	return descs
}

// drainMetrics runs fn with a buffered chan and returns everything it emits.
func drainMetrics(fn func(chan<- prometheus.Metric)) []prometheus.Metric {
	ch := make(chan prometheus.Metric, 128)
	go func() {
		fn(ch)
		close(ch)
	}()
	var ms []prometheus.Metric
	for m := range ch {
		ms = append(ms, m)
	}
	return ms
}

// text wraps a Go string in a Valid pgtype.Text for convenient test fixtures.
func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// int8v wraps a Go int64 in a Valid pgtype.Int8.
func int8v(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }

// floatv wraps a Go float64 in a Valid pgtype.Float8.
func floatv(v float64) pgtype.Float8 { return pgtype.Float8{Float64: v, Valid: true} }
