package collectors

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatWalReceiverCollector collects from pg_stat_wal_receiver (standby-
// side). On primaries the view is empty and this collector emits nothing.
//
// Labels: database, status, slot_name, sender_host.
type PgStatWalReceiverCollector struct {
	dbClients []*db.Client

	receiveStartLsn    *prometheus.GaugeVec
	writtenLsn         *prometheus.GaugeVec
	flushedLsn         *prometheus.GaugeVec
	latestEndLsn       *prometheus.GaugeVec
	lastMsgSendTime    *prometheus.GaugeVec
	lastMsgReceiptTime *prometheus.GaugeVec
	latestEndTime      *prometheus.GaugeVec
}

// NewPgStatWalReceiverCollector instantiates a new PgStatWalReceiverCollector.
func NewPgStatWalReceiverCollector(dbClients []*db.Client) *PgStatWalReceiverCollector {
	labels := []string{"database", "status", "slot_name", "sender_host"}
	gauge := gaugeFactory(walReceiverSubSystem, labels)
	return &PgStatWalReceiverCollector{
		dbClients: dbClients,

		receiveStartLsn:    gauge("receive_start_lsn_bytes", "WAL location at which streaming started, as a byte offset"),
		writtenLsn:         gauge("written_lsn_bytes", "Last WAL byte written to disk by the receiver"),
		flushedLsn:         gauge("flushed_lsn_bytes", "Last WAL byte flushed to disk by the receiver"),
		latestEndLsn:       gauge("latest_end_lsn_bytes", "Last WAL byte reported to the upstream by the receiver"),
		lastMsgSendTime:    gauge("last_msg_send_timestamp_seconds", "Unix time when the last message was sent from the upstream"),
		lastMsgReceiptTime: gauge("last_msg_receipt_timestamp_seconds", "Unix time when the last message was received from the upstream"),
		latestEndTime:      gauge("latest_end_timestamp_seconds", "Unix time of the last message the receiver replied to"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatWalReceiverCollector) Describe(ch chan<- *prometheus.Desc) {
	c.receiveStartLsn.Describe(ch)
	c.writtenLsn.Describe(ch)
	c.flushedLsn.Describe(ch)
	c.latestEndLsn.Describe(ch)
	c.lastMsgSendTime.Describe(ch)
	c.lastMsgReceiptTime.Describe(ch)
	c.latestEndTime.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatWalReceiverCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(gctx, dbClient) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatWalReceiverCollector) collectInto(ch chan<- prometheus.Metric) {
	c.receiveStartLsn.Collect(ch)
	c.writtenLsn.Collect(ch)
	c.flushedLsn.Collect(ch)
	c.latestEndLsn.Collect(ch)
	c.lastMsgSendTime.Collect(ch)
	c.lastMsgReceiptTime.Collect(ch)
	c.latestEndTime.Collect(ch)
}

func (c *PgStatWalReceiverCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatWalReceiver(ctx)
	if err != nil {
		return fmt.Errorf("wal_receiver stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatWalReceiverCollector) emit(stats []*model.PgStatWalReceiver) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.Status.String,
			stat.SlotName.String,
			stat.SenderHost.String,
		}
		emitFloat := func(vec *prometheus.GaugeVec, v pgtype.Float8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(v.Float64)
			}
		}
		emitTime := func(vec *prometheus.GaugeVec, v pgtype.Timestamptz) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(float64(v.Time.Unix()))
			}
		}
		emitFloat(c.receiveStartLsn, stat.ReceiveStartLsnBytes)
		emitFloat(c.writtenLsn, stat.WrittenLsnBytes)
		emitFloat(c.flushedLsn, stat.FlushedLsnBytes)
		emitFloat(c.latestEndLsn, stat.LatestEndLsnBytes)
		emitTime(c.lastMsgSendTime, stat.LastMsgSendTime)
		emitTime(c.lastMsgReceiptTime, stat.LastMsgReceiptTime)
		emitTime(c.latestEndTime, stat.LatestEndTime)
	}
}
