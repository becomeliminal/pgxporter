package collectors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatActivityCollector collects from pg_stat_user_tables.
type PgStatActivityCollector struct {
	dbClients []*db.Client
	mutex     sync.RWMutex

	activityCount *prometheus.Desc
	maxTxDuration *prometheus.Desc
}

// NewPgStatActivityCollector instantiates and returns a new PgStatActivityCollector.
func NewPgStatActivityCollector(dbClients []*db.Client) *PgStatActivityCollector {
	variableLabels := []string{"database", "datname", "state"}
	return &PgStatActivityCollector{
		dbClients: dbClients,

		activityCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, activitySubSystem, "count"),
			"Number of connections in this state",
			variableLabels,
			nil,
		),
		maxTxDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, activitySubSystem, "max_tx_duration"),
			"Max duration in seconds any active transaction has been running",
			variableLabels,
			nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatActivityCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.activityCount
	ch <- c.maxTxDuration
}

// Scrape implements our Scraper interface.
func (c *PgStatActivityCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("activity scrape took %dms", time.Now().Sub(start).Milliseconds())
	}()
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

func (c *PgStatActivityCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	activityStats, err := dbClient.SelectPgStatActivity(ctx)
	if err != nil {
		return fmt.Errorf("activity stats: %w", err)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.emit(activityStats, ch)
	return nil
}

// emit converts a slice of pg_stat_activity aggregation rows into metrics on ch.
// Valid gate per-field so partial-NULL rows still contribute what they can.
// Separated from scrape so it can be unit-tested without a live database.
func (c *PgStatActivityCollector) emit(stats []*model.PgStatActivity, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		if stat.Count.Valid {
			ch <- prometheus.MustNewConstMetric(c.activityCount, prometheus.GaugeValue,
				float64(stat.Count.Int64),
				stat.Database.String, stat.DatName.String, stat.State.String)
		}
		if stat.MaxTxDuration.Valid {
			ch <- prometheus.MustNewConstMetric(c.maxTxDuration, prometheus.GaugeValue,
				stat.MaxTxDuration.Float64,
				stat.Database.String, stat.DatName.String, stat.State.String)
		}
	}
}
