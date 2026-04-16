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

// PgStatIOUserIndexesCollector collects from pg_statio_user_indexes.
type PgStatIOUserIndexesCollector struct {
	dbClients []*db.Client
	mutex     sync.RWMutex

	idxBlksRead *prometheus.Desc
	idxBlksHit  *prometheus.Desc
}

// NewPgStatIOUserIndexesCollector instantiates and returns a new PgStatIOUserIndexesCollector.
func NewPgStatIOUserIndexesCollector(dbClients []*db.Client) *PgStatIOUserIndexesCollector {
	variableLabels := []string{"database", "schemaname", "relname", "indexrelname"}
	return &PgStatIOUserIndexesCollector{
		dbClients: dbClients,

		idxBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userIndexesSubSystem, "idx_blks_read"),
			"Number of disk blocks read from this index",
			variableLabels,
			nil,
		),
		idxBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userIndexesSubSystem, "idx_blks_hit"),
			"Number of buffer hits in this index",
			variableLabels,
			nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatIOUserIndexesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.idxBlksRead
	ch <- c.idxBlksHit
}

// Collect implements the promtheus.Collector.
func (c *PgStatIOUserIndexesCollector) Collect(ch chan<- prometheus.Metric) {
	_ = c.Scrape(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatIOUserIndexesCollector) Scrape(ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("I/O user index scrape took %dms", time.Now().Sub(start).Milliseconds())
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

func (c *PgStatIOUserIndexesCollector) scrape(dbClient *db.Client, ch chan<- prometheus.Metric) error {
	userIndexesStats, err := dbClient.SelectPgStatIOUserIndexes(context.Background())
	if err != nil {
		return fmt.Errorf("user table stats: %w", err)
	}
	for _, stat := range userIndexesStats {
		ch <- prometheus.MustNewConstMetric(c.idxBlksRead, prometheus.CounterValue, float64(stat.IndexBlksRead), stat.Database, stat.SchemaName, stat.RelName, stat.IndexRelName)
		ch <- prometheus.MustNewConstMetric(c.idxBlksHit, prometheus.CounterValue, float64(stat.IndexBlksHit), stat.Database, stat.SchemaName, stat.RelName, stat.IndexRelName)
	}
	return nil
}
