package collectors

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgSettingsCollector exports a snapshot of numeric-typed settings from
// pg_settings. String / enum settings are filtered server-side.
//
// Default-off: ~400 rows per database is real cardinality for most
// deployments. Users opt in explicitly.
type PgSettingsCollector struct {
	dbClients []*db.Client

	value *prometheus.GaugeVec
}

// NewPgSettingsCollector instantiates a new PgSettingsCollector.
func NewPgSettingsCollector(dbClients []*db.Client) *PgSettingsCollector {
	labels := []string{"database", "name", "unit", "vartype"}
	gauge := gaugeFactoryRawPg(settingsSubSystem, labels)
	return &PgSettingsCollector{
		dbClients: dbClients,
		value:     gauge("value", "Value of the pg_settings entry (bool as 0/1, integer/real as numeric)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgSettingsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.value.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgSettingsCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgSettingsCollector) collectInto(ch chan<- prometheus.Metric) {
	c.value.Collect(ch)
}

func (c *PgSettingsCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgSettings(ctx)
	if err != nil {
		return fmt.Errorf("pg_settings: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgSettingsCollector) emit(stats []*model.PgSetting) {
	for _, stat := range stats {
		if !stat.Name.Valid || !stat.Value.Valid {
			continue
		}
		c.value.WithLabelValues(
			stat.Database.String,
			stat.Name.String,
			stat.Unit.String,
			stat.VarType.String,
		).Set(stat.Value.Float64)
	}
}
