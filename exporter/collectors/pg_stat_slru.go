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

	blksZeroed  *counterDelta
	blksHit     *counterDelta
	blksRead    *counterDelta
	blksWritten *counterDelta
	blksExists  *counterDelta
	flushes     *counterDelta
	truncates   *counterDelta
	statsReset  *prometheus.GaugeVec
}

// NewPgStatSLRUCollector instantiates a new PgStatSLRUCollector.
func NewPgStatSLRUCollector(dbClients []*db.Client) *PgStatSLRUCollector {
	labels := []string{"database", "name"}
	counter := counterFactory(slruSubSystem, labels)
	gauge := gaugeFactory(slruSubSystem, labels)
	return &PgStatSLRUCollector{
		dbClients: dbClients,

		blksZeroed:  counter("blks_zeroed_total", "Blocks zeroed during initialisation"),
		blksHit:     counter("blks_hit_total", "Times a disk block was found in the SLRU"),
		blksRead:    counter("blks_read_total", "Disk blocks read for the SLRU"),
		blksWritten: counter("blks_written_total", "Disk blocks written for the SLRU"),
		blksExists:  counter("blks_exists_total", "Blocks checked for existence for the SLRU"),
		flushes:     counter("flushes_total", "SLRU flushes issued"),
		truncates:   counter("truncates_total", "SLRU truncates issued"),
		statsReset:  gauge("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatSLRUCollector) Describe(ch chan<- *prometheus.Desc) {
	c.blksZeroed.Describe(ch)
	c.blksHit.Describe(ch)
	c.blksRead.Describe(ch)
	c.blksWritten.Describe(ch)
	c.blksExists.Describe(ch)
	c.flushes.Describe(ch)
	c.truncates.Describe(ch)
	c.statsReset.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatSLRUCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatSLRUCollector) collectInto(ch chan<- prometheus.Metric) {
	c.blksZeroed.Collect(ch)
	c.blksHit.Collect(ch)
	c.blksRead.Collect(ch)
	c.blksWritten.Collect(ch)
	c.blksExists.Collect(ch)
	c.flushes.Collect(ch)
	c.truncates.Collect(ch)
	c.statsReset.Collect(ch)
}

func (c *PgStatSLRUCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatSLRU(ctx)
	if err != nil {
		return fmt.Errorf("slru stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatSLRUCollector) emit(stats []*model.PgStatSLRU) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.Name.String}
		emitCounter := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), labels...)
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
			c.statsReset.WithLabelValues(labels...).Set(float64(stat.StatsReset.Time.Unix()))
		}
	}
}
