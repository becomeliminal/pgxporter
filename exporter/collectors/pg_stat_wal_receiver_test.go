package collectors

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatWalReceiverCollector_Describe(t *testing.T) {
	c := NewPgStatWalReceiverCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 7; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatWalReceiverCollector_Emit(t *testing.T) {
	c := NewPgStatWalReceiverCollector(nil)
	now := time.Unix(1_700_000_000, 0)
	stats := []*model.PgStatWalReceiver{{
		Database:             text("postgres"),
		Status:               text("streaming"),
		SlotName:             text("main_slot"),
		SenderHost:           text("primary.example"),
		ReceiveStartLsnBytes: floatv(1024),
		WrittenLsnBytes:      floatv(2048),
		FlushedLsnBytes:      floatv(2000),
		LatestEndLsnBytes:    floatv(2048),
		LastMsgSendTime:      pgtype.Timestamptz{Time: now, Valid: true},
		LastMsgReceiptTime:   pgtype.Timestamptz{Time: now, Valid: true},
		LatestEndTime:        pgtype.Timestamptz{Time: now, Valid: true},
	}}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 7; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatWalReceiverCollector_Emit_EmptyOnPrimary(t *testing.T) {
	c := NewPgStatWalReceiverCollector(nil)
	c.emit(nil)
	ms := drainMetrics(c.collectInto)
	if got := len(ms); got != 0 {
		t.Errorf("primary (no rows) emitted %d metrics, want 0", got)
	}
}
