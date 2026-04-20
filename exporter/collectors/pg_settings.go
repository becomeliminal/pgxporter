package collectors

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgSettingsCollector emits numeric + bool settings from pg_settings.
// Labelled single metric family rather than postgres_exporter's
// one-metric-per-setting shape — the label-based layout is cleaner
// against cardinality budgeting and avoids a 300-descriptor init hit.
//
// Labels: database, name, unit (native GUC unit or empty), vartype
// (bool | integer | real).
type PgSettingsCollector struct {
	dbClients []*db.Client
	value     *prometheus.Desc
}

// NewPgSettingsCollector instantiates a new PgSettingsCollector.
func NewPgSettingsCollector(dbClients []*db.Client) *PgSettingsCollector {
	labels := []string{"database", "name", "unit", "vartype"}
	return &PgSettingsCollector{
		dbClients: dbClients,
		value: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceRawPg, "settings", "value"),
			"Value of a numeric or bool setting from pg_settings",
			labels, nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgSettingsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.value
}

// Scrape implements our Scraper interface.
func (c *PgSettingsCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgSettingsCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgSettings(ctx)
	if err != nil {
		return fmt.Errorf("pg_settings: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgSettingsCollector) emit(stats []*model.PgSetting, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		if !stat.Value.Valid {
			continue
		}
		ch <- prometheus.MustNewConstMetric(c.value, prometheus.GaugeValue,
			stat.Value.Float64,
			stat.Database.String, stat.Name.String, stat.Unit.String, stat.VarType.String,
		)
	}
}
