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

// PgStatWalCollector collects cluster-wide WAL generation stats (PG 14+).
// Pre-14 servers have no such view → the db layer returns nil and no
// metrics emit.
type PgStatWalCollector struct {
	dbClients []*db.Client

	walRecords     *counterDelta
	walFpi         *counterDelta
	walBytes       *counterDelta
	walBuffersFull *counterDelta
	walWrite       *counterDelta // PG 14 only
	walSync        *counterDelta // PG 14 only
	walWriteTime   *counterDelta // PG 14 only
	walSyncTime    *counterDelta // PG 14 only
	statsReset     *prometheus.GaugeVec
}

// NewPgStatWalCollector instantiates a new PgStatWalCollector.
func NewPgStatWalCollector(dbClients []*db.Client) *PgStatWalCollector {
	labels := []string{"database"}
	counter := counterFactory(walSubSystem, labels)
	gauge := gaugeFactory(walSubSystem, labels)
	return &PgStatWalCollector{
		dbClients: dbClients,

		walRecords:     counter("records_total", "WAL records generated"),
		walFpi:         counter("fpi_total", "WAL full-page images generated"),
		walBytes:       counter("bytes_total", "Total WAL bytes generated"),
		walBuffersFull: counter("buffers_full_total", "Times WAL data was written because buffers were full"),
		walWrite:       counter("write_total", "WAL writes issued (PG 14 only — merged into pg_stat_io in PG 15)"),
		walSync:        counter("sync_total", "WAL syncs issued (PG 14 only)"),
		walWriteTime:   counter("write_time_seconds_total", "Total time spent in WAL writes, seconds (PG 14 only)"),
		walSyncTime:    counter("sync_time_seconds_total", "Total time spent in WAL syncs, seconds (PG 14 only)"),
		statsReset:     gauge("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatWalCollector) Describe(ch chan<- *prometheus.Desc) {
	c.walRecords.Describe(ch)
	c.walFpi.Describe(ch)
	c.walBytes.Describe(ch)
	c.walBuffersFull.Describe(ch)
	c.walWrite.Describe(ch)
	c.walSync.Describe(ch)
	c.walWriteTime.Describe(ch)
	c.walSyncTime.Describe(ch)
	c.statsReset.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatWalCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatWalCollector) collectInto(ch chan<- prometheus.Metric) {
	c.walRecords.Collect(ch)
	c.walFpi.Collect(ch)
	c.walBytes.Collect(ch)
	c.walBuffersFull.Collect(ch)
	c.walWrite.Collect(ch)
	c.walSync.Collect(ch)
	c.walWriteTime.Collect(ch)
	c.walSyncTime.Collect(ch)
	c.statsReset.Collect(ch)
}

func (c *PgStatWalCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatWal(ctx)
	if err != nil {
		return fmt.Errorf("wal stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatWalCollector) emit(stats []*model.PgStatWal) {
	for _, stat := range stats {
		database := stat.Database.String
		emitCounterInt := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), database)
			}
		}
		emitCounterFloat := func(cd *counterDelta, v pgtype.Float8) {
			if v.Valid {
				cd.Observe(v.Float64, database)
			}
		}
		emitMillisAsSecs := func(cd *counterDelta, v pgtype.Float8) {
			if v.Valid {
				cd.Observe(v.Float64/1000.0, database)
			}
		}
		emitTime := func(vec *prometheus.GaugeVec, v pgtype.Timestamptz) {
			if v.Valid {
				vec.WithLabelValues(database).Set(float64(v.Time.Unix()))
			}
		}
		emitCounterInt(c.walRecords, stat.WalRecords)
		emitCounterInt(c.walFpi, stat.WalFpi)
		emitCounterFloat(c.walBytes, stat.WalBytes)
		emitCounterInt(c.walBuffersFull, stat.WalBuffersFull)
		emitCounterInt(c.walWrite, stat.WalWrite)
		emitCounterInt(c.walSync, stat.WalSync)
		emitMillisAsSecs(c.walWriteTime, stat.WalWriteTime)
		emitMillisAsSecs(c.walSyncTime, stat.WalSyncTime)
		emitTime(c.statsReset, stat.StatsReset)
	}
}
