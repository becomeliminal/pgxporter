package collectors

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgDatabaseSizeCollector emits pg_database_size() per non-template database.
// A single gauge: pg_database_size_bytes{database, datname}.
type PgDatabaseSizeCollector struct {
	dbClients []*db.Client
	sizeBytes *prometheus.GaugeVec
}

// NewPgDatabaseSizeCollector instantiates a new PgDatabaseSizeCollector.
func NewPgDatabaseSizeCollector(dbClients []*db.Client) *PgDatabaseSizeCollector {
	labels := []string{"database", "datname"}
	gauge := gaugeFactoryRawPg("database", labels)
	return &PgDatabaseSizeCollector{
		dbClients: dbClients,
		sizeBytes: gauge("size_bytes", "Disk space used by the database, in bytes"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgDatabaseSizeCollector) Describe(ch chan<- *prometheus.Desc) {
	c.sizeBytes.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgDatabaseSizeCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgDatabaseSizeCollector) collectInto(ch chan<- prometheus.Metric) {
	c.sizeBytes.Collect(ch)
}

func (c *PgDatabaseSizeCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgDatabaseSize(ctx)
	if err != nil {
		return fmt.Errorf("database size: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgDatabaseSizeCollector) emit(stats []*model.PgDatabaseSize) {
	for _, stat := range stats {
		if stat.Bytes.Valid {
			c.sizeBytes.WithLabelValues(stat.Database.String, stat.DatName.String).
				Set(float64(stat.Bytes.Int64))
		}
	}
}
