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

	numTimed           *prometheus.Desc
	numRequested       *prometheus.Desc
	restartpointsTimed *prometheus.Desc
	restartpointsReq   *prometheus.Desc
	restartpointsDone  *prometheus.Desc
	writeTime          *prometheus.Desc // seconds
	syncTime           *prometheus.Desc // seconds
	buffersWritten     *prometheus.Desc
	statsReset         *prometheus.Desc
}

// NewPgStatCheckpointerCollector instantiates a new PgStatCheckpointerCollector.
func NewPgStatCheckpointerCollector(dbClients []*db.Client) *PgStatCheckpointerCollector {
	labels := []string{"database"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, checkpointerSubSystem, name), help, labels, nil)
	}
	return &PgStatCheckpointerCollector{
		dbClients: dbClients,

		numTimed:           desc("num_timed_total", "Scheduled checkpoints performed"),
		numRequested:       desc("num_requested_total", "Requested (non-scheduled) checkpoints performed"),
		restartpointsTimed: desc("restartpoints_timed_total", "Scheduled restartpoints performed (standby)"),
		restartpointsReq:   desc("restartpoints_req_total", "Requested restartpoints performed (standby)"),
		restartpointsDone:  desc("restartpoints_done_total", "Restartpoints that were successfully completed (standby)"),
		writeTime:          desc("write_time_seconds_total", "Total time spent writing files to disk during checkpoints, seconds"),
		syncTime:           desc("sync_time_seconds_total", "Total time spent synchronizing files to disk during checkpoints, seconds"),
		buffersWritten:     desc("buffers_written_total", "Buffers written during checkpoints"),
		statsReset:         desc("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatCheckpointerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.numTimed
	ch <- c.numRequested
	ch <- c.restartpointsTimed
	ch <- c.restartpointsReq
	ch <- c.restartpointsDone
	ch <- c.writeTime
	ch <- c.syncTime
	ch <- c.buffersWritten
	ch <- c.statsReset
}

// Scrape implements our Scraper interface.
func (c *PgStatCheckpointerCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatCheckpointerCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatCheckpointer(ctx)
	if err != nil {
		return fmt.Errorf("checkpointer stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatCheckpointerCollector) emit(stats []*model.PgStatCheckpointer, ch chan<- prometheus.Metric) {
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
		emitInt(c.numTimed, prometheus.CounterValue, stat.NumTimed)
		emitInt(c.numRequested, prometheus.CounterValue, stat.NumRequested)
		emitInt(c.restartpointsTimed, prometheus.CounterValue, stat.RestartpointsTimed)
		emitInt(c.restartpointsReq, prometheus.CounterValue, stat.RestartpointsReq)
		emitInt(c.restartpointsDone, prometheus.CounterValue, stat.RestartpointsDone)
		emitMillisAsSecs(c.writeTime, stat.WriteTime)
		emitMillisAsSecs(c.syncTime, stat.SyncTime)
		emitInt(c.buffersWritten, prometheus.CounterValue, stat.BuffersWritten)
		emitTime(c.statsReset, stat.StatsReset)
	}
}
