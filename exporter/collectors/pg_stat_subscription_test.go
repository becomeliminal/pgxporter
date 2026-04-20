package collectors

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatSubscriptionCollector_Describe(t *testing.T) {
	c := NewPgStatSubscriptionCollector(nil)
	// 6 gauges from pg_stat_subscription + 2 counters + 1 gauge from
	// pg_stat_subscription_stats = 9.
	if got, want := len(drainDescs(c.Describe)), 9; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatSubscriptionCollector_Emit(t *testing.T) {
	c := NewPgStatSubscriptionCollector(nil)
	now := time.Unix(1_700_000_000, 0)
	subs := []*model.PgStatSubscription{{
		Database:           text("postgres"),
		SubName:            text("sub1"),
		Active:             pgtype.Bool{Bool: true, Valid: true},
		ReceivedLsnBytes:   int8v(123456789),
		LastMsgSendTime:    pgtype.Timestamptz{Time: now, Valid: true},
		LastMsgReceiptTime: pgtype.Timestamptz{Time: now, Valid: true},
		LatestEndLsnBytes:  int8v(123456000),
		LatestEndTime:      pgtype.Timestamptz{Time: now, Valid: true},
	}}
	c.emit(subs)

	stats := []*model.PgStatSubscriptionStats{{
		Database:        text("postgres"),
		SubName:         text("sub1"),
		ApplyErrorCount: int8v(2),
		SyncErrorCount:  int8v(0),
		StatsReset:      pgtype.Timestamptz{Time: now, Valid: true},
	}}
	c.emitStats(stats)

	ms := drainMetrics(c.collectInto)
	// 6 gauges from pg_stat_subscription + 2 counters + 1 stats_reset = 9.
	if got, want := len(ms), 9; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatSubscriptionCollector_Emit_Inactive(t *testing.T) {
	c := NewPgStatSubscriptionCollector(nil)
	// Subscription defined but no worker running — active=false, the LSN
	// and time fields are NULL (no worker has reported).
	subs := []*model.PgStatSubscription{{
		Database: text("postgres"),
		SubName:  text("sub1"),
		Active:   pgtype.Bool{Bool: false, Valid: true},
	}}
	c.emit(subs)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 1; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
