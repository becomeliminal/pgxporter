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

// PgStatCheckpointerCollector collects from pg_stat_checkpointer (PG 17+).
//
// First-mover advantage: postgres_exporter doesn't have a collector for this
// view yet. Enabled by default; on pre-17 servers the underlying SELECT is
// skipped by the db layer and no metrics emit.
type PgStatCheckpointerCollector struct {
	dbClients []*db.Client

	numTimed           *counterDelta
	numRequested       *counterDelta
	restartpointsTimed *counterDelta
	restartpointsReq   *counterDelta
	restartpointsDone  *counterDelta
	writeTime          *counterDelta // seconds
	syncTime           *counterDelta // seconds
	buffersWritten     *counterDelta
	statsReset         *prometheus.GaugeVec
}

// NewPgStatCheckpointerCollector instantiates a new PgStatCheckpointerCollector.
func NewPgStatCheckpointerCollector(dbClients []*db.Client) *PgStatCheckpointerCollector {
	labels := []string{"database"}
	counter := counterFactory(checkpointerSubSystem, labels)
	gauge := gaugeFactory(checkpointerSubSystem, labels)
	return &PgStatCheckpointerCollector{
		dbClients: dbClients,

		numTimed:           counter("num_timed_total", "Scheduled checkpoints performed"),
		numRequested:       counter("num_requested_total", "Requested (non-scheduled) checkpoints performed"),
		restartpointsTimed: counter("restartpoints_timed_total", "Scheduled restartpoints performed (standby)"),
		restartpointsReq:   counter("restartpoints_req_total", "Requested restartpoints performed (standby)"),
		restartpointsDone:  counter("restartpoints_done_total", "Restartpoints that were successfully completed (standby)"),
		writeTime:          counter("write_time_seconds_total", "Total time spent writing files to disk during checkpoints, seconds"),
		syncTime:           counter("sync_time_seconds_total", "Total time spent synchronizing files to disk during checkpoints, seconds"),
		buffersWritten:     counter("buffers_written_total", "Buffers written during checkpoints"),
		statsReset:         gauge("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatCheckpointerCollector) Describe(ch chan<- *prometheus.Desc) {
	c.numTimed.Describe(ch)
	c.numRequested.Describe(ch)
	c.restartpointsTimed.Describe(ch)
	c.restartpointsReq.Describe(ch)
	c.restartpointsDone.Describe(ch)
	c.writeTime.Describe(ch)
	c.syncTime.Describe(ch)
	c.buffersWritten.Describe(ch)
	c.statsReset.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatCheckpointerCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatCheckpointerCollector) collectInto(ch chan<- prometheus.Metric) {
	c.numTimed.Collect(ch)
	c.numRequested.Collect(ch)
	c.restartpointsTimed.Collect(ch)
	c.restartpointsReq.Collect(ch)
	c.restartpointsDone.Collect(ch)
	c.writeTime.Collect(ch)
	c.syncTime.Collect(ch)
	c.buffersWritten.Collect(ch)
	c.statsReset.Collect(ch)
}

func (c *PgStatCheckpointerCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatCheckpointer(ctx)
	if err != nil {
		return fmt.Errorf("checkpointer stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatCheckpointerCollector) emit(stats []*model.PgStatCheckpointer) {
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
		emitCounter(c.numTimed, stat.NumTimed)
		emitCounter(c.numRequested, stat.NumRequested)
		emitCounter(c.restartpointsTimed, stat.RestartpointsTimed)
		emitCounter(c.restartpointsReq, stat.RestartpointsReq)
		emitCounter(c.restartpointsDone, stat.RestartpointsDone)
		emitMillisAsSecs(c.writeTime, stat.WriteTime)
		emitMillisAsSecs(c.syncTime, stat.SyncTime)
		emitCounter(c.buffersWritten, stat.BuffersWritten)
		emitTime(c.statsReset, stat.StatsReset)
	}
}
