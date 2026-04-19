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

// PgStatBgwriterCollector collects from pg_stat_bgwriter.
//
// PG 17 moved the checkpoint and backend-buffer columns out of this view
// into pg_stat_checkpointer. We still emit the same metric names — the
// checkpoint-related metrics simply won't appear for PG 17+ servers (the
// dedicated pg_stat_checkpointer collector covers those separately).
type PgStatBgwriterCollector struct {
	dbClients []*db.Client

	// All PG versions
	buffersClean    *prometheus.Desc
	maxwrittenClean *prometheus.Desc
	buffersAlloc    *prometheus.Desc
	statsReset      *prometheus.Desc

	// PG < 17 only
	checkpointsTimed    *prometheus.Desc
	checkpointsReq      *prometheus.Desc
	checkpointWriteTime *prometheus.Desc // seconds (converted from ms)
	checkpointSyncTime  *prometheus.Desc // seconds
	buffersCheckpoint   *prometheus.Desc
	buffersBackend      *prometheus.Desc
	buffersBackendFsync *prometheus.Desc
}

// NewPgStatBgwriterCollector instantiates and returns a new PgStatBgwriterCollector.
func NewPgStatBgwriterCollector(dbClients []*db.Client) *PgStatBgwriterCollector {
	labels := []string{"database"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, bgwriterSubSystem, name), help, labels, nil)
	}
	return &PgStatBgwriterCollector{
		dbClients: dbClients,

		buffersClean:    desc("buffers_clean_total", "Buffers written by the background writer"),
		maxwrittenClean: desc("maxwritten_clean_total", "Times the background writer stopped a cleaning scan because it had written too many buffers"),
		buffersAlloc:    desc("buffers_alloc_total", "Buffers allocated"),
		statsReset:      desc("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),

		checkpointsTimed:    desc("checkpoints_timed_total", "Scheduled checkpoints performed (PG <17 only — see pg_stat_checkpointer on PG 17+)"),
		checkpointsReq:      desc("checkpoints_req_total", "Requested (non-scheduled) checkpoints performed (PG <17)"),
		checkpointWriteTime: desc("checkpoint_write_time_seconds_total", "Total time spent writing files to disk during checkpoints, seconds (PG <17)"),
		checkpointSyncTime:  desc("checkpoint_sync_time_seconds_total", "Total time spent synchronizing files to disk during checkpoints, seconds (PG <17)"),
		buffersCheckpoint:   desc("buffers_checkpoint_total", "Buffers written during checkpoints (PG <17)"),
		buffersBackend:      desc("buffers_backend_total", "Buffers written directly by a backend (PG <17)"),
		buffersBackendFsync: desc("buffers_backend_fsync_total", "Times a backend had to execute its own fsync (PG <17)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatBgwriterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.buffersClean
	ch <- c.maxwrittenClean
	ch <- c.buffersAlloc
	ch <- c.statsReset
	ch <- c.checkpointsTimed
	ch <- c.checkpointsReq
	ch <- c.checkpointWriteTime
	ch <- c.checkpointSyncTime
	ch <- c.buffersCheckpoint
	ch <- c.buffersBackend
	ch <- c.buffersBackendFsync
}

// Scrape implements our Scraper interface.
func (c *PgStatBgwriterCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("bgwriter scrape took %dms", time.Since(start).Milliseconds())
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

func (c *PgStatBgwriterCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatBgwriter(ctx)
	if err != nil {
		return fmt.Errorf("bgwriter stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatBgwriterCollector) emit(stats []*model.PgStatBgwriter, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		database := stat.Database.String
		emitInt := func(desc *prometheus.Desc, valueType prometheus.ValueType, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, valueType, float64(v.Int64), database)
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

		emitInt(c.buffersClean, prometheus.CounterValue, stat.BuffersClean)
		emitInt(c.maxwrittenClean, prometheus.CounterValue, stat.MaxwrittenClean)
		emitInt(c.buffersAlloc, prometheus.CounterValue, stat.BuffersAlloc)
		emitTime(c.statsReset, stat.StatsReset)

		emitInt(c.checkpointsTimed, prometheus.CounterValue, stat.CheckpointsTimed)
		emitInt(c.checkpointsReq, prometheus.CounterValue, stat.CheckpointsReq)
		emitMillisAsSecs(c.checkpointWriteTime, stat.CheckpointWriteTime)
		emitMillisAsSecs(c.checkpointSyncTime, stat.CheckpointSyncTime)
		emitInt(c.buffersCheckpoint, prometheus.CounterValue, stat.BuffersCheckpoint)
		emitInt(c.buffersBackend, prometheus.CounterValue, stat.BuffersBackend)
		emitInt(c.buffersBackendFsync, prometheus.CounterValue, stat.BuffersBackendFsync)
	}
}
