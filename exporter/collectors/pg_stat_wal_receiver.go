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

	receiveStartLsn    *prometheus.Desc
	writtenLsn         *prometheus.Desc
	flushedLsn         *prometheus.Desc
	latestEndLsn       *prometheus.Desc
	lastMsgSendTime    *prometheus.Desc
	lastMsgReceiptTime *prometheus.Desc
	latestEndTime      *prometheus.Desc
}

// NewPgStatWalReceiverCollector instantiates a new PgStatWalReceiverCollector.
func NewPgStatWalReceiverCollector(dbClients []*db.Client) *PgStatWalReceiverCollector {
	labels := []string{"database", "status", "slot_name", "sender_host"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, walReceiverSubSystem, name), help, labels, nil)
	}
	return &PgStatWalReceiverCollector{
		dbClients: dbClients,

		receiveStartLsn:    desc("receive_start_lsn_bytes", "WAL location at which streaming started, as a byte offset"),
		writtenLsn:         desc("written_lsn_bytes", "Last WAL byte written to disk by the receiver"),
		flushedLsn:         desc("flushed_lsn_bytes", "Last WAL byte flushed to disk by the receiver"),
		latestEndLsn:       desc("latest_end_lsn_bytes", "Last WAL byte reported to the upstream by the receiver"),
		lastMsgSendTime:    desc("last_msg_send_timestamp_seconds", "Unix time when the last message was sent from the upstream"),
		lastMsgReceiptTime: desc("last_msg_receipt_timestamp_seconds", "Unix time when the last message was received from the upstream"),
		latestEndTime:      desc("latest_end_timestamp_seconds", "Unix time of the last message the receiver replied to"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatWalReceiverCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.receiveStartLsn
	ch <- c.writtenLsn
	ch <- c.flushedLsn
	ch <- c.latestEndLsn
	ch <- c.lastMsgSendTime
	ch <- c.lastMsgReceiptTime
	ch <- c.latestEndTime
}

// Scrape implements our Scraper interface.
func (c *PgStatWalReceiverCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(gctx, dbClient, ch) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatWalReceiverCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatWalReceiver(ctx)
	if err != nil {
		return fmt.Errorf("wal_receiver stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatWalReceiverCollector) emit(stats []*model.PgStatWalReceiver, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.Status.String,
			stat.SlotName.String,
			stat.SenderHost.String,
		}
		emitFloat := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v.Float64, labels...)
			}
		}
		emitTime := func(desc *prometheus.Desc, v pgtype.Timestamptz) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Time.Unix()), labels...)
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
