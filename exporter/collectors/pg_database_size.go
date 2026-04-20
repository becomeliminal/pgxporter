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

// PgDatabaseSizeCollector emits pg_database_size() per non-template database.
// A single gauge: pg_database_size_bytes{database, datname}.
type PgDatabaseSizeCollector struct {
	dbClients []*db.Client
	sizeBytes *prometheus.Desc
}

// NewPgDatabaseSizeCollector instantiates a new PgDatabaseSizeCollector.
func NewPgDatabaseSizeCollector(dbClients []*db.Client) *PgDatabaseSizeCollector {
	labels := []string{"database", "datname"}
	return &PgDatabaseSizeCollector{
		dbClients: dbClients,
		sizeBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceRawPg, "database", "size_bytes"),
			"Disk space used by the database, in bytes",
			labels, nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgDatabaseSizeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.sizeBytes
}

// Scrape implements our Scraper interface.
func (c *PgDatabaseSizeCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgDatabaseSizeCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgDatabaseSize(ctx)
	if err != nil {
		return fmt.Errorf("database size: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgDatabaseSizeCollector) emit(stats []*model.PgDatabaseSize, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		emitInt := func(desc *prometheus.Desc, v pgtype.Int8, labels ...string) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Int64), labels...)
			}
		}
		emitInt(c.sizeBytes, stat.Bytes, stat.Database.String, stat.DatName.String)
	}
}
