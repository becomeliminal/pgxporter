package collectors

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatUserIndexesCollector collects from pg_stat_user_indexes.
type PgStatUserIndexesCollector struct {
	dbClients []*db.Client

	idxScan     *counterDelta
	idxTupRead  *counterDelta
	idxTupFetch *counterDelta
}

// NewPgStatUserIndexesCollector instantiates and returns a new PgStatUserIndexesCollector.
func NewPgStatUserIndexesCollector(dbClients []*db.Client) *PgStatUserIndexesCollector {
	variableLabels := []string{"database", "schemaname", "relname", "indexrelname"}
	counter := counterFactory(userIndexesSubSystem, variableLabels)
	return &PgStatUserIndexesCollector{
		dbClients:   dbClients,
		idxScan:     counter("index_scan", "Number of index scans initiated on this index"),
		idxTupRead:  counter("index_tup_read", "Number of index entries returned by scans on this index"),
		idxTupFetch: counter("index_tup_fetch", "Number of live table rows fetched by simple index scans using this index"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatUserIndexesCollector) Describe(ch chan<- *prometheus.Desc) {
	c.idxScan.Describe(ch)
	c.idxTupRead.Describe(ch)
	c.idxTupFetch.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatUserIndexesCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatUserIndexesCollector) collectInto(ch chan<- prometheus.Metric) {
	c.idxScan.Collect(ch)
	c.idxTupRead.Collect(ch)
	c.idxTupFetch.Collect(ch)
}

func (c *PgStatUserIndexesCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	userIndexStats, err := dbClient.SelectPgStatUserIndexes(ctx)
	if err != nil {
		return fmt.Errorf("user indexes stats: %w", err)
	}
	c.emit(userIndexStats)
	return nil
}

// emit turns scanned pg_stat_user_indexes rows into metrics, skipping NULL
// counter columns. Separated from scrape for unit-test coverage.
func (c *PgStatUserIndexesCollector) emit(stats []*model.PgStatUserIndex) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.SchemaName.String, stat.RelName.String, stat.IndexRelName.String}
		if stat.IndexScan.Valid {
			c.idxScan.Observe(float64(stat.IndexScan.Int64), labels...)
		}
		if stat.IndexTupRead.Valid {
			c.idxTupRead.Observe(float64(stat.IndexTupRead.Int64), labels...)
		}
		if stat.IndexTupFetch.Valid {
			c.idxTupFetch.Observe(float64(stat.IndexTupFetch.Int64), labels...)
		}
	}
}
