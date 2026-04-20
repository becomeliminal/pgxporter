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

// PgStatSLRUCollector emits SLRU buffer-pool stats (PG 13+). Returns nil
// rows on pre-13 servers so no metrics emit. First-mover: postgres_exporter
// doesn't ship this collector.
type PgStatSLRUCollector struct {
	dbClients []*db.Client

	blksZeroed  *prometheus.Desc
	blksHit     *prometheus.Desc
	blksRead    *prometheus.Desc
	blksWritten *prometheus.Desc
	blksExists  *prometheus.Desc
	flushes     *prometheus.Desc
	truncates   *prometheus.Desc
	statsReset  *prometheus.Desc
}

// NewPgStatSLRUCollector instantiates a new PgStatSLRUCollector.
func NewPgStatSLRUCollector(dbClients []*db.Client) *PgStatSLRUCollector {
	labels := []string{"database", "name"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, slruSubSystem, name), help, labels, nil)
	}
	return &PgStatSLRUCollector{
		dbClients: dbClients,

		blksZeroed:  desc("blks_zeroed_total", "Blocks zeroed during initialisation"),
		blksHit:     desc("blks_hit_total", "Times a disk block was found in the SLRU"),
		blksRead:    desc("blks_read_total", "Disk blocks read for the SLRU"),
		blksWritten: desc("blks_written_total", "Disk blocks written for the SLRU"),
		blksExists:  desc("blks_exists_total", "Blocks checked for existence for the SLRU"),
		flushes:     desc("flushes_total", "SLRU flushes issued"),
		truncates:   desc("truncates_total", "SLRU truncates issued"),
		statsReset:  desc("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatSLRUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.blksZeroed
	ch <- c.blksHit
	ch <- c.blksRead
	ch <- c.blksWritten
	ch <- c.blksExists
	ch <- c.flushes
	ch <- c.truncates
	ch <- c.statsReset
}

// Scrape implements our Scraper interface.
func (c *PgStatSLRUCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatSLRUCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatSLRU(ctx)
	if err != nil {
		return fmt.Errorf("slru stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatSLRUCollector) emit(stats []*model.PgStatSLRU, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.Name.String}
		emitCounter := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(v.Int64), labels...)
			}
		}
		emitCounter(c.blksZeroed, stat.BlksZeroed)
		emitCounter(c.blksHit, stat.BlksHit)
		emitCounter(c.blksRead, stat.BlksRead)
		emitCounter(c.blksWritten, stat.BlksWritten)
		emitCounter(c.blksExists, stat.BlksExists)
		emitCounter(c.flushes, stat.Flushes)
		emitCounter(c.truncates, stat.Truncates)
		if stat.StatsReset.Valid {
			ch <- prometheus.MustNewConstMetric(c.statsReset, prometheus.GaugeValue, float64(stat.StatsReset.Time.Unix()), labels...)
		}
	}
}
