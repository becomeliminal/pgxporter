package collectors

import (
	"context"
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatSSLCollector collects aggregate TLS connection counts from
// pg_stat_ssl. Rows are grouped by (ssl, version, cipher, bits) to avoid
// per-pid cardinality.
type PgStatSSLCollector struct {
	dbClients []*db.Client

	connections *prometheus.GaugeVec
}

// NewPgStatSSLCollector instantiates a new PgStatSSLCollector.
func NewPgStatSSLCollector(dbClients []*db.Client) *PgStatSSLCollector {
	labels := []string{"database", "ssl", "version", "cipher", "bits"}
	gauge := gaugeFactory(sslSubSystem, labels)
	return &PgStatSSLCollector{
		dbClients:   dbClients,
		connections: gauge("connections", "Number of connections grouped by TLS parameters"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatSSLCollector) Describe(ch chan<- *prometheus.Desc) {
	c.connections.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatSSLCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatSSLCollector) collectInto(ch chan<- prometheus.Metric) {
	c.connections.Collect(ch)
}

func (c *PgStatSSLCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatSSL(ctx)
	if err != nil {
		return fmt.Errorf("ssl stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatSSLCollector) emit(stats []*model.PgStatSSL) {
	for _, stat := range stats {
		if !stat.SSL.Valid || !stat.Connections.Valid {
			continue
		}
		c.connections.WithLabelValues(
			stat.Database.String,
			strconv.FormatBool(stat.SSL.Bool),
			stat.Version.String,
			stat.Cipher.String,
			strconv.Itoa(int(stat.Bits.Int64)),
		).Set(float64(stat.Connections.Int64))
	}
}
