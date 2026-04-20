package collectors

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatSubscriptionCollector_Describe(t *testing.T) {
	c := NewPgStatSubscriptionCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 5; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatSubscriptionCollector_Emit(t *testing.T) {
	c := NewPgStatSubscriptionCollector(nil)
	now := time.Unix(1_700_000_000, 0)
	stats := []*model.PgStatSubscription{{
		Database:           text("postgres"),
		SubName:            text("mysub"),
		WorkerType:         text("main"),
		ReceivedLsnBytes:   floatv(1024),
		LastMsgSendTime:    pgtype.Timestamptz{Time: now, Valid: true},
		LastMsgReceiptTime: pgtype.Timestamptz{Time: now, Valid: true},
		LatestEndLsnBytes:  floatv(1024),
		LatestEndTime:      pgtype.Timestamptz{Time: now, Valid: true},
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 5; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatSubscriptionCollector_Empty(t *testing.T) {
	c := NewPgStatSubscriptionCollector(nil)
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(nil, ch) })
	if got := len(ms); got != 0 {
		t.Errorf("empty rows emitted %d metrics, want 0", got)
	}
}
