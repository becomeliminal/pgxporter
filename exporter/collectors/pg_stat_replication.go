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

// PgStatReplicationCollector collects per-replica replication stats from the
// primary's pg_stat_replication view. On standbys the view is usually empty
// and the collector emits no metrics.
//
// Labels: database, application_name, client_addr, state, sync_state. The
// state / sync_state labels are low-cardinality enums; application_name and
// client_addr identify the replica.
type PgStatReplicationCollector struct {
	dbClients []*db.Client

	sentLagBytes     *prometheus.GaugeVec
	writeLagBytes    *prometheus.GaugeVec
	flushLagBytes    *prometheus.GaugeVec
	replayLagBytes   *prometheus.GaugeVec
	writeLagSeconds  *prometheus.GaugeVec
	flushLagSeconds  *prometheus.GaugeVec
	replayLagSeconds *prometheus.GaugeVec
}

// NewPgStatReplicationCollector instantiates a new PgStatReplicationCollector.
func NewPgStatReplicationCollector(dbClients []*db.Client) *PgStatReplicationCollector {
	labels := []string{"database", "application_name", "client_addr", "state", "sync_state"}
	gauge := gaugeFactory(replicationSubSystem, labels)
	return &PgStatReplicationCollector{
		dbClients: dbClients,

		sentLagBytes:     gauge("sent_lag_bytes", "WAL bytes generated on primary but not yet sent to replica"),
		writeLagBytes:    gauge("write_lag_bytes", "WAL bytes sent to replica but not yet written to its OS buffers"),
		flushLagBytes:    gauge("flush_lag_bytes", "WAL bytes written on replica but not yet flushed to disk"),
		replayLagBytes:   gauge("replay_lag_bytes", "WAL bytes flushed on replica but not yet replayed"),
		writeLagSeconds:  gauge("write_lag_seconds", "Time elapsed between local flush and receipt of write confirmation from replica"),
		flushLagSeconds:  gauge("flush_lag_seconds", "Time elapsed between local flush and receipt of flush confirmation from replica"),
		replayLagSeconds: gauge("replay_lag_seconds", "Time elapsed between local flush and receipt of replay confirmation from replica"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatReplicationCollector) Describe(ch chan<- *prometheus.Desc) {
	c.sentLagBytes.Describe(ch)
	c.writeLagBytes.Describe(ch)
	c.flushLagBytes.Describe(ch)
	c.replayLagBytes.Describe(ch)
	c.writeLagSeconds.Describe(ch)
	c.flushLagSeconds.Describe(ch)
	c.replayLagSeconds.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatReplicationCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatReplicationCollector) collectInto(ch chan<- prometheus.Metric) {
	c.sentLagBytes.Collect(ch)
	c.writeLagBytes.Collect(ch)
	c.flushLagBytes.Collect(ch)
	c.replayLagBytes.Collect(ch)
	c.writeLagSeconds.Collect(ch)
	c.flushLagSeconds.Collect(ch)
	c.replayLagSeconds.Collect(ch)
}

func (c *PgStatReplicationCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatReplication(ctx)
	if err != nil {
		return fmt.Errorf("replication stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatReplicationCollector) emit(stats []*model.PgStatReplication) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.ApplicationName.String,
			stat.ClientAddr.String,
			stat.State.String,
			stat.SyncState.String,
		}
		emitFloat := func(vec *prometheus.GaugeVec, v pgtype.Float8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(v.Float64)
			}
		}
		emitFloat(c.sentLagBytes, stat.SentLagBytes)
		emitFloat(c.writeLagBytes, stat.WriteLagBytes)
		emitFloat(c.flushLagBytes, stat.FlushLagBytes)
		emitFloat(c.replayLagBytes, stat.ReplayLagBytes)
		emitFloat(c.writeLagSeconds, stat.WriteLagSeconds)
		emitFloat(c.flushLagSeconds, stat.FlushLagSeconds)
		emitFloat(c.replayLagSeconds, stat.ReplayLagSeconds)
	}
}
