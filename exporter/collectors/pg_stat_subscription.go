package collectors

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatSubscriptionCollector collects logical-replication subscriber
// state from pg_stat_subscription (per-subname aggregate) and, on PG 15+,
// error counters from pg_stat_subscription_stats.
//
// Default-off: logical replication is niche enough that most deployments
// don't use it and don't want the label cardinality.
type PgStatSubscriptionCollector struct {
	dbClients []*db.Client

	active             *prometheus.GaugeVec
	receivedLsn        *prometheus.GaugeVec
	lastMsgSendTime    *prometheus.GaugeVec
	lastMsgReceiptTime *prometheus.GaugeVec
	latestEndLsn       *prometheus.GaugeVec
	latestEndTime      *prometheus.GaugeVec

	applyErrorCount  *counterDelta
	syncErrorCount   *counterDelta
	statsResetSecond *prometheus.GaugeVec
}

// NewPgStatSubscriptionCollector instantiates a new PgStatSubscriptionCollector.
func NewPgStatSubscriptionCollector(dbClients []*db.Client) *PgStatSubscriptionCollector {
	labels := []string{"database", "subname"}
	gauge := gaugeFactory(subscriptionSubSystem, labels)
	counter := counterFactory(subscriptionSubSystem, labels)
	return &PgStatSubscriptionCollector{
		dbClients:          dbClients,
		active:             gauge("active", "1 if any worker for the subscription is running, 0 otherwise"),
		receivedLsn:        gauge("received_lsn_bytes", "Last write-ahead log location received, as a byte offset"),
		lastMsgSendTime:    gauge("last_msg_send_time_seconds", "Send time of the last message received from the origin"),
		lastMsgReceiptTime: gauge("last_msg_receipt_time_seconds", "Receipt time of the last message received from the origin"),
		latestEndLsn:       gauge("latest_end_lsn_bytes", "Last WAL location reported to the origin, as a byte offset"),
		latestEndTime:      gauge("latest_end_time_seconds", "Time of the last message reported to the origin"),
		applyErrorCount:    counter("apply_error_count_total", "Number of times an error occurred while applying changes"),
		syncErrorCount:     counter("sync_error_count_total", "Number of times an error occurred during initial table sync"),
		statsResetSecond:   gauge("stats_reset_timestamp_seconds", "Unix time at which subscription stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatSubscriptionCollector) Describe(ch chan<- *prometheus.Desc) {
	c.active.Describe(ch)
	c.receivedLsn.Describe(ch)
	c.lastMsgSendTime.Describe(ch)
	c.lastMsgReceiptTime.Describe(ch)
	c.latestEndLsn.Describe(ch)
	c.latestEndTime.Describe(ch)
	c.applyErrorCount.Describe(ch)
	c.syncErrorCount.Describe(ch)
	c.statsResetSecond.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatSubscriptionCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatSubscriptionCollector) collectInto(ch chan<- prometheus.Metric) {
	c.active.Collect(ch)
	c.receivedLsn.Collect(ch)
	c.lastMsgSendTime.Collect(ch)
	c.lastMsgReceiptTime.Collect(ch)
	c.latestEndLsn.Collect(ch)
	c.latestEndTime.Collect(ch)
	c.applyErrorCount.Collect(ch)
	c.syncErrorCount.Collect(ch)
	c.statsResetSecond.Collect(ch)
}

func (c *PgStatSubscriptionCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	subs, err := dbClient.SelectPgStatSubscription(ctx)
	if err != nil {
		return fmt.Errorf("subscription stats: %w", err)
	}
	c.emit(subs)

	statsRows, err := dbClient.SelectPgStatSubscriptionStats(ctx)
	if err != nil {
		return fmt.Errorf("subscription_stats: %w", err)
	}
	c.emitStats(statsRows)
	return nil
}

func (c *PgStatSubscriptionCollector) emit(stats []*model.PgStatSubscription) {
	for _, stat := range stats {
		if !stat.Database.Valid || !stat.SubName.Valid {
			continue
		}
		database, subname := stat.Database.String, stat.SubName.String
		if stat.Active.Valid {
			active := 0.0
			if stat.Active.Bool {
				active = 1.0
			}
			c.active.WithLabelValues(database, subname).Set(active)
		}
		if stat.ReceivedLsnBytes.Valid {
			c.receivedLsn.WithLabelValues(database, subname).Set(float64(stat.ReceivedLsnBytes.Int64))
		}
		if stat.LastMsgSendTime.Valid {
			c.lastMsgSendTime.WithLabelValues(database, subname).Set(float64(stat.LastMsgSendTime.Time.Unix()))
		}
		if stat.LastMsgReceiptTime.Valid {
			c.lastMsgReceiptTime.WithLabelValues(database, subname).Set(float64(stat.LastMsgReceiptTime.Time.Unix()))
		}
		if stat.LatestEndLsnBytes.Valid {
			c.latestEndLsn.WithLabelValues(database, subname).Set(float64(stat.LatestEndLsnBytes.Int64))
		}
		if stat.LatestEndTime.Valid {
			c.latestEndTime.WithLabelValues(database, subname).Set(float64(stat.LatestEndTime.Time.Unix()))
		}
	}
}

func (c *PgStatSubscriptionCollector) emitStats(stats []*model.PgStatSubscriptionStats) {
	for _, stat := range stats {
		if !stat.Database.Valid || !stat.SubName.Valid {
			continue
		}
		database, subname := stat.Database.String, stat.SubName.String
		if stat.ApplyErrorCount.Valid {
			c.applyErrorCount.Observe(float64(stat.ApplyErrorCount.Int64), database, subname)
		}
		if stat.SyncErrorCount.Valid {
			c.syncErrorCount.Observe(float64(stat.SyncErrorCount.Int64), database, subname)
		}
		if stat.StatsReset.Valid {
			c.statsResetSecond.WithLabelValues(database, subname).Set(float64(stat.StatsReset.Time.Unix()))
		}
	}
}
