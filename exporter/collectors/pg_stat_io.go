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

// PgStatIOCollector emits cross-cutting I/O stats (PG 16+). Each row is a
// (backend_type, object, context) bucket — e.g. (autovacuum worker,
// relation, vacuum) — and many counters are NULL on buckets where they
// don't apply (per-Valid gating skips those).
//
// postgres_exporter doesn't ship a collector for this view; pgxporter
// is first-mover.
type PgStatIOCollector struct {
	dbClients []*db.Client

	reads         *prometheus.Desc
	readTime      *prometheus.Desc
	writes        *prometheus.Desc
	writeTime     *prometheus.Desc
	writebacks    *prometheus.Desc
	writebackTime *prometheus.Desc
	extends       *prometheus.Desc
	extendTime    *prometheus.Desc
	opBytes       *prometheus.Desc
	hits          *prometheus.Desc
	evictions     *prometheus.Desc
	reuses        *prometheus.Desc
	fsyncs        *prometheus.Desc
	fsyncTime     *prometheus.Desc
	statsReset    *prometheus.Desc
}

// NewPgStatIOCollector instantiates a new PgStatIOCollector.
func NewPgStatIOCollector(dbClients []*db.Client) *PgStatIOCollector {
	labels := []string{"database", "backend_type", "object", "context"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, ioSubSystem, name), help, labels, nil)
	}
	return &PgStatIOCollector{
		dbClients: dbClients,

		reads:         desc("reads_total", "Read operations issued"),
		readTime:      desc("read_time_seconds_total", "Total time spent in read operations, seconds"),
		writes:        desc("writes_total", "Write operations issued"),
		writeTime:     desc("write_time_seconds_total", "Total time spent in write operations, seconds"),
		writebacks:    desc("writebacks_total", "Writeback operations (dirty buffers flushed)"),
		writebackTime: desc("writeback_time_seconds_total", "Total time spent in writeback operations, seconds"),
		extends:       desc("extends_total", "Relation-extension operations"),
		extendTime:    desc("extend_time_seconds_total", "Total time spent extending relations, seconds"),
		opBytes:       desc("op_bytes", "Block size for I/O operations in this bucket, bytes (typically 8192)"),
		hits:          desc("hits_total", "Shared-buffer hits"),
		evictions:     desc("evictions_total", "Buffers evicted to make room"),
		reuses:        desc("reuses_total", "Buffers reused from a previous strategy ring"),
		fsyncs:        desc("fsyncs_total", "fsync operations issued"),
		fsyncTime:     desc("fsync_time_seconds_total", "Total time spent in fsync operations, seconds"),
		statsReset:    desc("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatIOCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.reads
	ch <- c.readTime
	ch <- c.writes
	ch <- c.writeTime
	ch <- c.writebacks
	ch <- c.writebackTime
	ch <- c.extends
	ch <- c.extendTime
	ch <- c.opBytes
	ch <- c.hits
	ch <- c.evictions
	ch <- c.reuses
	ch <- c.fsyncs
	ch <- c.fsyncTime
	ch <- c.statsReset
}

// Scrape implements our Scraper interface.
func (c *PgStatIOCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatIOCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatIO(ctx)
	if err != nil {
		return fmt.Errorf("io stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatIOCollector) emit(stats []*model.PgStatIO, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.BackendType.String,
			stat.Object.String,
			stat.Context.String,
		}
		emitCounter := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(v.Int64), labels...)
			}
		}
		emitGauge := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Int64), labels...)
			}
		}
		emitMillisAsSecs := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, v.Float64/1000.0, labels...)
			}
		}
		emitTime := func(desc *prometheus.Desc, v pgtype.Timestamptz) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Time.Unix()), labels...)
			}
		}
		emitCounter(c.reads, stat.Reads)
		emitMillisAsSecs(c.readTime, stat.ReadTime)
		emitCounter(c.writes, stat.Writes)
		emitMillisAsSecs(c.writeTime, stat.WriteTime)
		emitCounter(c.writebacks, stat.Writebacks)
		emitMillisAsSecs(c.writebackTime, stat.WritebackTime)
		emitCounter(c.extends, stat.Extends)
		emitMillisAsSecs(c.extendTime, stat.ExtendTime)
		emitGauge(c.opBytes, stat.OpBytes)
		emitCounter(c.hits, stat.Hits)
		emitCounter(c.evictions, stat.Evictions)
		emitCounter(c.reuses, stat.Reuses)
		emitCounter(c.fsyncs, stat.Fsyncs)
		emitMillisAsSecs(c.fsyncTime, stat.FsyncTime)
		emitTime(c.statsReset, stat.StatsReset)
	}
}
