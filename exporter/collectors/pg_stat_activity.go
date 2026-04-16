package collectors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
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

// Collect implements the promtheus.Collector.
func (c *PgStatActivityCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_ = c.Scrape(ch)
}

// Scrape implements our Scraper interfacc.
func (c *PgStatActivityCollector) Scrape(ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("activity scrape took %dms", time.Now().Sub(start).Milliseconds())
	}()
	group := errgroup.Group{}
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(dbClient, ch) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatActivityCollector) scrape(dbClient *db.Client, ch chan<- prometheus.Metric) error {
	activityStats, err := dbClient.SelectPgStatActivity(context.Background())
	if err != nil {
		return fmt.Errorf("activity stats: %w", err)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, stat := range activityStats {
		ch <- prometheus.MustNewConstMetric(c.activityCount, prometheus.GaugeValue, float64(stat.Count), stat.Database, stat.DatName, stat.State)
		ch <- prometheus.MustNewConstMetric(c.maxTxDuration, prometheus.GaugeValue, stat.MaxTxDuration, stat.Database, stat.DatName, stat.State)
	}
	return nil
}
