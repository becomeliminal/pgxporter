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

// PgStatSSLCollector emits aggregate connection counts by TLS state.
// Useful for verifying TLS rollouts and catching legacy-cipher clients.
//
// Labels: database, ssl (true/false), version, cipher, bits.
type PgStatSSLCollector struct {
	dbClients   []*db.Client
	connections *prometheus.Desc
}

// NewPgStatSSLCollector instantiates a new PgStatSSLCollector.
func NewPgStatSSLCollector(dbClients []*db.Client) *PgStatSSLCollector {
	labels := []string{"database", "ssl", "version", "cipher", "bits"}
	return &PgStatSSLCollector{
		dbClients: dbClients,
		connections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, sslSubSystem, "connections"),
			"Backend connections grouped by TLS state",
			labels, nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatSSLCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.connections
}

// Scrape implements our Scraper interface.
func (c *PgStatSSLCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatSSLCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatSSL(ctx)
	if err != nil {
		return fmt.Errorf("ssl stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatSSLCollector) emit(stats []*model.PgStatSSL, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		if !stat.Count.Valid {
			continue
		}
		ssl := ""
		if stat.SSL.Valid {
			ssl = strconv.FormatBool(stat.SSL.Bool)
		}
		bits := ""
		if stat.Bits.Valid {
			bits = strconv.FormatInt(stat.Bits.Int64, 10)
		}
		ch <- prometheus.MustNewConstMetric(c.connections, prometheus.GaugeValue,
			float64(stat.Count.Int64),
			stat.Database.String, ssl, stat.Version.String, stat.Cipher.String, bits,
		)
	}
}
