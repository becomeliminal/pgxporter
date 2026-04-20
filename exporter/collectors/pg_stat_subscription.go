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

// PgStatSubscriptionCollector emits subscriber-side logical-replication
// state. Zero rows (and therefore zero metrics) on any server that
// isn't a subscriber. postgres_exporter doesn't ship a collector for
// this view.
//
// Labels: database, subname, worker_type.
type PgStatSubscriptionCollector struct {
	dbClients []*db.Client

	receivedLsn        *prometheus.Desc
	lastMsgSendTime    *prometheus.Desc
	lastMsgReceiptTime *prometheus.Desc
	latestEndLsn       *prometheus.Desc
	latestEndTime      *prometheus.Desc
}

// NewPgStatSubscriptionCollector instantiates a new PgStatSubscriptionCollector.
func NewPgStatSubscriptionCollector(dbClients []*db.Client) *PgStatSubscriptionCollector {
	labels := []string{"database", "subname", "worker_type"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, subscriptionSubSystem, name), help, labels, nil)
	}
	return &PgStatSubscriptionCollector{
		dbClients: dbClients,

		receivedLsn:        desc("received_lsn_bytes", "Byte offset of the last WAL location received"),
		lastMsgSendTime:    desc("last_msg_send_timestamp_seconds", "Unix time the last message was sent by the upstream"),
		lastMsgReceiptTime: desc("last_msg_receipt_timestamp_seconds", "Unix time the last message was received by this subscriber"),
		latestEndLsn:       desc("latest_end_lsn_bytes", "Byte offset of the last WAL location reported to the upstream"),
		latestEndTime:      desc("latest_end_timestamp_seconds", "Unix time of the last message the subscriber replied to"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatSubscriptionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.receivedLsn
	ch <- c.lastMsgSendTime
	ch <- c.lastMsgReceiptTime
	ch <- c.latestEndLsn
	ch <- c.latestEndTime
}

// Scrape implements our Scraper interface.
func (c *PgStatSubscriptionCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatSubscriptionCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatSubscription(ctx)
	if err != nil {
		return fmt.Errorf("subscription stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatSubscriptionCollector) emit(stats []*model.PgStatSubscription, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.SubName.String, stat.WorkerType.String}
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
		emitFloat(c.receivedLsn, stat.ReceivedLsnBytes)
		emitTime(c.lastMsgSendTime, stat.LastMsgSendTime)
		emitTime(c.lastMsgReceiptTime, stat.LastMsgReceiptTime)
		emitFloat(c.latestEndLsn, stat.LatestEndLsnBytes)
		emitTime(c.latestEndTime, stat.LatestEndTime)
	}
}
