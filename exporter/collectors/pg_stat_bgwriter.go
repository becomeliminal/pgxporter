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

// PgStatBgwriterCollector collects from pg_stat_bgwriter.
//
// PG 17 moved the checkpoint and backend-buffer columns out of this view
// into pg_stat_checkpointer. We still emit the same metric names — the
// checkpoint-related metrics simply won't appear for PG 17+ servers (the
// dedicated pg_stat_checkpointer collector covers those separately).
type PgStatBgwriterCollector struct {
	dbClients []*db.Client

	// All PG versions
	buffersClean    *counterDelta
	maxwrittenClean *counterDelta
	buffersAlloc    *counterDelta
	statsReset      *prometheus.GaugeVec

	// PG < 17 only
	checkpointsTimed    *counterDelta
	checkpointsReq      *counterDelta
	checkpointWriteTime *counterDelta // seconds (converted from ms)
	checkpointSyncTime  *counterDelta // seconds
	buffersCheckpoint   *counterDelta
	buffersBackend      *counterDelta
	buffersBackendFsync *counterDelta
}

// NewPgStatBgwriterCollector instantiates and returns a new PgStatBgwriterCollector.
func NewPgStatBgwriterCollector(dbClients []*db.Client) *PgStatBgwriterCollector {
	labels := []string{"database"}
	counter := counterFactory(bgwriterSubSystem, labels)
	gauge := gaugeFactory(bgwriterSubSystem, labels)
	return &PgStatBgwriterCollector{
		dbClients: dbClients,

		buffersClean:    counter("buffers_clean_total", "Buffers written by the background writer"),
		maxwrittenClean: counter("maxwritten_clean_total", "Times the background writer stopped a cleaning scan because it had written too many buffers"),
		buffersAlloc:    counter("buffers_alloc_total", "Buffers allocated"),
		statsReset:      gauge("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),

		checkpointsTimed:    counter("checkpoints_timed_total", "Scheduled checkpoints performed (PG <17 only — see pg_stat_checkpointer on PG 17+)"),
		checkpointsReq:      counter("checkpoints_req_total", "Requested (non-scheduled) checkpoints performed (PG <17)"),
		checkpointWriteTime: counter("checkpoint_write_time_seconds_total", "Total time spent writing files to disk during checkpoints, seconds (PG <17)"),
		checkpointSyncTime:  counter("checkpoint_sync_time_seconds_total", "Total time spent synchronizing files to disk during checkpoints, seconds (PG <17)"),
		buffersCheckpoint:   counter("buffers_checkpoint_total", "Buffers written during checkpoints (PG <17)"),
		buffersBackend:      counter("buffers_backend_total", "Buffers written directly by a backend (PG <17)"),
		buffersBackendFsync: counter("buffers_backend_fsync_total", "Times a backend had to execute its own fsync (PG <17)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatBgwriterCollector) Describe(ch chan<- *prometheus.Desc) {
	c.buffersClean.Describe(ch)
	c.maxwrittenClean.Describe(ch)
	c.buffersAlloc.Describe(ch)
	c.statsReset.Describe(ch)
	c.checkpointsTimed.Describe(ch)
	c.checkpointsReq.Describe(ch)
	c.checkpointWriteTime.Describe(ch)
	c.checkpointSyncTime.Describe(ch)
	c.buffersCheckpoint.Describe(ch)
	c.buffersBackend.Describe(ch)
	c.buffersBackendFsync.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatBgwriterCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatBgwriterCollector) collectInto(ch chan<- prometheus.Metric) {
	c.buffersClean.Collect(ch)
	c.maxwrittenClean.Collect(ch)
	c.buffersAlloc.Collect(ch)
	c.statsReset.Collect(ch)
	c.checkpointsTimed.Collect(ch)
	c.checkpointsReq.Collect(ch)
	c.checkpointWriteTime.Collect(ch)
	c.checkpointSyncTime.Collect(ch)
	c.buffersCheckpoint.Collect(ch)
	c.buffersBackend.Collect(ch)
	c.buffersBackendFsync.Collect(ch)
}

func (c *PgStatBgwriterCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatBgwriter(ctx)
	if err != nil {
		return fmt.Errorf("bgwriter stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatBgwriterCollector) emit(stats []*model.PgStatBgwriter) {
	for _, stat := range stats {
		database := stat.Database.String
		emitCounter := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), database)
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

		emitCounter(c.buffersClean, stat.BuffersClean)
		emitCounter(c.maxwrittenClean, stat.MaxwrittenClean)
		emitCounter(c.buffersAlloc, stat.BuffersAlloc)
		emitTime(c.statsReset, stat.StatsReset)

		emitCounter(c.checkpointsTimed, stat.CheckpointsTimed)
		emitCounter(c.checkpointsReq, stat.CheckpointsReq)
		emitMillisAsSecs(c.checkpointWriteTime, stat.CheckpointWriteTime)
		emitMillisAsSecs(c.checkpointSyncTime, stat.CheckpointSyncTime)
		emitCounter(c.buffersCheckpoint, stat.BuffersCheckpoint)
		emitCounter(c.buffersBackend, stat.BuffersBackend)
		emitCounter(c.buffersBackendFsync, stat.BuffersBackendFsync)
	}
}
