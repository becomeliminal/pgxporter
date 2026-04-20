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

	sentLagBytes     *prometheus.Desc
	writeLagBytes    *prometheus.Desc
	flushLagBytes    *prometheus.Desc
	replayLagBytes   *prometheus.Desc
	writeLagSeconds  *prometheus.Desc
	flushLagSeconds  *prometheus.Desc
	replayLagSeconds *prometheus.Desc
}

// NewPgStatReplicationCollector instantiates a new PgStatReplicationCollector.
func NewPgStatReplicationCollector(dbClients []*db.Client) *PgStatReplicationCollector {
	labels := []string{"database", "application_name", "client_addr", "state", "sync_state"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, replicationSubSystem, name), help, labels, nil)
	}
	return &PgStatReplicationCollector{
		dbClients: dbClients,

		sentLagBytes:     desc("sent_lag_bytes", "WAL bytes generated on primary but not yet sent to replica"),
		writeLagBytes:    desc("write_lag_bytes", "WAL bytes sent to replica but not yet written to its OS buffers"),
		flushLagBytes:    desc("flush_lag_bytes", "WAL bytes written on replica but not yet flushed to disk"),
		replayLagBytes:   desc("replay_lag_bytes", "WAL bytes flushed on replica but not yet replayed"),
		writeLagSeconds:  desc("write_lag_seconds", "Time elapsed between local flush and receipt of write confirmation from replica"),
		flushLagSeconds:  desc("flush_lag_seconds", "Time elapsed between local flush and receipt of flush confirmation from replica"),
		replayLagSeconds: desc("replay_lag_seconds", "Time elapsed between local flush and receipt of replay confirmation from replica"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatReplicationCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.sentLagBytes
	ch <- c.writeLagBytes
	ch <- c.flushLagBytes
	ch <- c.replayLagBytes
	ch <- c.writeLagSeconds
	ch <- c.flushLagSeconds
	ch <- c.replayLagSeconds
}

// Scrape implements our Scraper interface.
func (c *PgStatReplicationCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatReplicationCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatReplication(ctx)
	if err != nil {
		return fmt.Errorf("replication stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatReplicationCollector) emit(stats []*model.PgStatReplication, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.ApplicationName.String,
			stat.ClientAddr.String,
			stat.State.String,
			stat.SyncState.String,
		}
		emitFloat := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v.Float64, labels...)
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
