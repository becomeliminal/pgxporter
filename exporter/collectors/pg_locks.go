package collectors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
)

// PgLocksCollector collects from pg_locks.
type PgLocksCollector struct {
	dbClients []*db.Client
	mutex     sync.RWMutex

	count *prometheus.Desc
}

// NewPgLocksCollector instantiates and returns a new PgLocksCollector.
func NewPgLocksCollector(dbClients []*db.Client) *PgLocksCollector {
	variableLabels := []string{"database", "datname", "mode"}
	return &PgLocksCollector{
		dbClients: dbClients,

		count: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, locksSubSystem, "count"),
			"Number of locks",
			variableLabels,
			nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgLocksCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.count
}

// Scrape implements our Scraper interface.
func (c *PgLocksCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("lock scrape took %dms", time.Now().Sub(start).Milliseconds())
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

func (c *PgLocksCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	locks, err := dbClient.SelectPgLocks(ctx)
	if err != nil {
		return fmt.Errorf("lock stats: %w", err)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, stat := range locks {
		if stat.Count.Valid {
			ch <- prometheus.MustNewConstMetric(c.count, prometheus.GaugeValue,
				float64(stat.Count.Int64),
				stat.Database.String, stat.DatName.String, stat.Mode.String)
		}
	}
	return nil
}
