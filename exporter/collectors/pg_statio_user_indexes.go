package collectors

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatIOUserIndexesCollector collects from pg_statio_user_indexes.
type PgStatIOUserIndexesCollector struct {
	dbClients []*db.Client

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

// Scrape implements our Scraper interface.
func (c *PgStatIOUserIndexesCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatIOUserIndexesCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	userIndexesStats, err := dbClient.SelectPgStatIOUserIndexes(ctx)
	if err != nil {
		return fmt.Errorf("user index stats: %w", err)
	}
	c.emit(userIndexesStats, ch)
	return nil
}

// emit turns scanned pg_statio_user_indexes rows into metrics, skipping NULL
// counter columns. Separated from scrape for unit-test coverage.
func (c *PgStatIOUserIndexesCollector) emit(stats []*model.PgStatIOUserIndex, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.SchemaName.String, stat.RelName.String, stat.IndexRelName.String}
		if stat.IndexBlksRead.Valid {
			ch <- prometheus.MustNewConstMetric(c.idxBlksRead, prometheus.CounterValue, float64(stat.IndexBlksRead.Int64), labels...)
		}
		if stat.IndexBlksHit.Valid {
			ch <- prometheus.MustNewConstMetric(c.idxBlksHit, prometheus.CounterValue, float64(stat.IndexBlksHit.Int64), labels...)
		}
	}
}
