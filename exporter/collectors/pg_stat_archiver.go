package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

	archivedCount    *prometheus.Desc
	lastArchivedTime *prometheus.Desc
	failedCount      *prometheus.Desc
	lastFailedTime   *prometheus.Desc
	statsReset       *prometheus.Desc
}

// NewPgStatArchiverCollector instantiates a new PgStatArchiverCollector.
func NewPgStatArchiverCollector(dbClients []*db.Client) *PgStatArchiverCollector {
	labels := []string{"database"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, archiverSubSystem, name), help, labels, nil)
	}
	return &PgStatArchiverCollector{
		dbClients: dbClients,

		archivedCount:    desc("archived_count_total", "WAL files successfully archived"),
		lastArchivedTime: desc("last_archived_timestamp_seconds", "Unix time of the most recent successful archive"),
		failedCount:      desc("failed_count_total", "Failed attempts to archive WAL files"),
		lastFailedTime:   desc("last_failed_timestamp_seconds", "Unix time of the most recent archive failure"),
		statsReset:       desc("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatArchiverCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.archivedCount
	ch <- c.lastArchivedTime
	ch <- c.failedCount
	ch <- c.lastFailedTime
	ch <- c.statsReset
}

// Scrape implements our Scraper interface.
func (c *PgStatArchiverCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("archiver scrape took %dms", time.Since(start).Milliseconds())
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

func (c *PgStatArchiverCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatArchiver(ctx)
	if err != nil {
		return fmt.Errorf("archiver stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatArchiverCollector) emit(stats []*model.PgStatArchiver, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		database := stat.Database.String
		emitInt := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(v.Int64), database)
			}
		}
		emitTime := func(desc *prometheus.Desc, v pgtype.Timestamptz) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Time.Unix()), database)
			}
		}
		emitInt(c.archivedCount, stat.ArchivedCount)
		emitTime(c.lastArchivedTime, stat.LastArchivedTime)
		emitInt(c.failedCount, stat.FailedCount)
		emitTime(c.lastFailedTime, stat.LastFailedTime)
		emitTime(c.statsReset, stat.StatsReset)
	}
}
