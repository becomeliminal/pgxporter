package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgLocksCollector_Describe(t *testing.T) {
	c := NewPgLocksCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 3; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgLocksCollector_Emit(t *testing.T) {
	c := NewPgLocksCollector(nil)
	locks := []*model.PgLock{
		{
			Database: text("postgres"), DatName: text("postgres"),
			Mode: text("AccessShareLock"), LockType: text("relation"),
			Granted: pgtype.Bool{Bool: true, Valid: true},
			Count:   int8v(4),
		},
		{
			Database: text("postgres"), DatName: text("postgres"),
			Mode: text("ExclusiveLock"), LockType: text("tuple"),
			Granted: pgtype.Bool{Bool: false, Valid: true},
			Count:   int8v(1),
		},
	}
	summary := []*model.PgLocksBlockingSummary{{
		Database:        text("postgres"),
		BlockedBackends: int8v(1),
		BlockerEdges:    int8v(2),
	}}
	c.emit(locks, summary)
	ms := drainMetrics(c.collectInto)
	// 2 lock-count rows + 2 summary gauges = 4
	if got, want := len(ms), 4; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgLocksCollector_Emit_NoLocksNoBlocking(t *testing.T) {
	c := NewPgLocksCollector(nil)
	summary := []*model.PgLocksBlockingSummary{{
		Database:        text("postgres"),
		BlockedBackends: int8v(0),
		BlockerEdges:    int8v(0),
	}}
	c.emit(nil, summary)
	ms := drainMetrics(c.collectInto)
	// 0 lock rows + 2 zero-valued summary gauges (still Valid) = 2
	if got, want := len(ms), 2; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgLocksCollector_Emit_NullCountSkipped(t *testing.T) {
	c := NewPgLocksCollector(nil)
	locks := []*model.PgLock{{
		Database: text("postgres"), DatName: text("postgres"),
		Mode: text("AccessShareLock"), LockType: text("relation"),
		Granted: pgtype.Bool{Bool: true, Valid: true},
		Count:   pgtype.Int8{},
	}}
	c.emit(locks, nil)
	ms := drainMetrics(c.collectInto)
	if got := len(ms); got != 0 {
		t.Errorf("emit produced %d metrics, want 0 (NULL count skipped)", got)
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
