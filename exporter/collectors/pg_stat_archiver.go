package collectors

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatArchiverCollector collects from pg_stat_archiver. The view is a
// single cluster-wide row describing WAL archiving progress — critical
// signal for any primary using continuous archiving.
type PgStatArchiverCollector struct {
	dbClients []*db.Client

	archivedCount    *counterDelta
	lastArchivedTime *prometheus.GaugeVec
	failedCount      *counterDelta
	lastFailedTime   *prometheus.GaugeVec
	statsReset       *prometheus.GaugeVec
}

// NewPgStatArchiverCollector instantiates a new PgStatArchiverCollector.
func NewPgStatArchiverCollector(dbClients []*db.Client) *PgStatArchiverCollector {
	labels := []string{"database"}
	counter := counterFactory(archiverSubSystem, labels)
	gauge := gaugeFactory(archiverSubSystem, labels)
	return &PgStatArchiverCollector{
		dbClients:        dbClients,
		archivedCount:    counter("archived_count_total", "WAL files successfully archived"),
		lastArchivedTime: gauge("last_archived_timestamp_seconds", "Unix time of the most recent successful archive"),
		failedCount:      counter("failed_count_total", "Failed attempts to archive WAL files"),
		lastFailedTime:   gauge("last_failed_timestamp_seconds", "Unix time of the most recent archive failure"),
		statsReset:       gauge("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatArchiverCollector) Describe(ch chan<- *prometheus.Desc) {
	c.archivedCount.Describe(ch)
	c.lastArchivedTime.Describe(ch)
	c.failedCount.Describe(ch)
	c.lastFailedTime.Describe(ch)
	c.statsReset.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatArchiverCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

// collectInto forwards every tracked child metric to ch. Separated from
// Scrape so unit tests can exercise emit → collect without a real DB.
func (c *PgStatArchiverCollector) collectInto(ch chan<- prometheus.Metric) {
	c.archivedCount.Collect(ch)
	c.lastArchivedTime.Collect(ch)
	c.failedCount.Collect(ch)
	c.lastFailedTime.Collect(ch)
	c.statsReset.Collect(ch)
}

func (c *PgStatArchiverCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatArchiver(ctx)
	if err != nil {
		return fmt.Errorf("archiver stats: %w", err)
	}
	c.emit(stats)
	return nil
}

// emit pushes each row's values into the collector's internal Vec
// state. Counter-typed values go through counterDelta so we preserve
// counter semantics despite PG exposing them as absolute cumulatives.
// NULL fields are skipped — no metric is emitted for them.
func (c *PgStatArchiverCollector) emit(stats []*model.PgStatArchiver) {
	for _, stat := range stats {
		database := stat.Database.String
		if stat.ArchivedCount.Valid {
			c.archivedCount.Observe(float64(stat.ArchivedCount.Int64), database)
		}
		if stat.LastArchivedTime.Valid {
			c.lastArchivedTime.WithLabelValues(database).Set(float64(stat.LastArchivedTime.Time.Unix()))
		}
		if stat.FailedCount.Valid {
			c.failedCount.Observe(float64(stat.FailedCount.Int64), database)
		}
		if stat.LastFailedTime.Valid {
			c.lastFailedTime.WithLabelValues(database).Set(float64(stat.LastFailedTime.Time.Unix()))
		}
		if stat.StatsReset.Valid {
			c.statsReset.WithLabelValues(database).Set(float64(stat.StatsReset.Time.Unix()))
		}
	}
}
