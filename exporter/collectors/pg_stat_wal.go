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

	walRecords     *prometheus.Desc
	walFpi         *prometheus.Desc
	walBytes       *prometheus.Desc
	walBuffersFull *prometheus.Desc
	walWrite       *prometheus.Desc // PG 14 only
	walSync        *prometheus.Desc // PG 14 only
	walWriteTime   *prometheus.Desc // PG 14 only
	walSyncTime    *prometheus.Desc // PG 14 only
	statsReset     *prometheus.Desc
}

// NewPgStatWalCollector instantiates a new PgStatWalCollector.
func NewPgStatWalCollector(dbClients []*db.Client) *PgStatWalCollector {
	labels := []string{"database"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, walSubSystem, name), help, labels, nil)
	}
	return &PgStatWalCollector{
		dbClients: dbClients,

		walRecords:     desc("records_total", "WAL records generated"),
		walFpi:         desc("fpi_total", "WAL full-page images generated"),
		walBytes:       desc("bytes_total", "Total WAL bytes generated"),
		walBuffersFull: desc("buffers_full_total", "Times WAL data was written because buffers were full"),
		walWrite:       desc("write_total", "WAL writes issued (PG 14 only — merged into pg_stat_io in PG 15)"),
		walSync:        desc("sync_total", "WAL syncs issued (PG 14 only)"),
		walWriteTime:   desc("write_time_seconds_total", "Total time spent in WAL writes, seconds (PG 14 only)"),
		walSyncTime:    desc("sync_time_seconds_total", "Total time spent in WAL syncs, seconds (PG 14 only)"),
		statsReset:     desc("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatWalCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.walRecords
	ch <- c.walFpi
	ch <- c.walBytes
	ch <- c.walBuffersFull
	ch <- c.walWrite
	ch <- c.walSync
	ch <- c.walWriteTime
	ch <- c.walSyncTime
	ch <- c.statsReset
}

// Scrape implements our Scraper interface.
func (c *PgStatWalCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatWalCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatWal(ctx)
	if err != nil {
		return fmt.Errorf("wal stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatWalCollector) emit(stats []*model.PgStatWal, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		database := stat.Database.String
		emitInt := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(v.Int64), database)
			}
		}
		emitFloat := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, v.Float64, database)
			}
		}
		emitMillisAsSecs := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, v.Float64/1000.0, database)
			}
		}
		emitTime := func(desc *prometheus.Desc, v pgtype.Timestamptz) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Time.Unix()), database)
			}
		}
		emitInt(c.walRecords, stat.WalRecords)
		emitInt(c.walFpi, stat.WalFpi)
		emitFloat(c.walBytes, stat.WalBytes)
		emitInt(c.walBuffersFull, stat.WalBuffersFull)
		emitInt(c.walWrite, stat.WalWrite)
		emitInt(c.walSync, stat.WalSync)
		emitMillisAsSecs(c.walWriteTime, stat.WalWriteTime)
		emitMillisAsSecs(c.walSyncTime, stat.WalSyncTime)
		emitTime(c.statsReset, stat.StatsReset)
	}
}
