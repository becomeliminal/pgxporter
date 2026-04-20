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

	reads         *counterDelta
	readTime      *counterDelta
	writes        *counterDelta
	writeTime     *counterDelta
	writebacks    *counterDelta
	writebackTime *counterDelta
	extends       *counterDelta
	extendTime    *counterDelta
	opBytes       *prometheus.GaugeVec
	hits          *counterDelta
	evictions     *counterDelta
	reuses        *counterDelta
	fsyncs        *counterDelta
	fsyncTime     *counterDelta
	statsReset    *prometheus.GaugeVec
}

// NewPgStatIOCollector instantiates a new PgStatIOCollector.
func NewPgStatIOCollector(dbClients []*db.Client) *PgStatIOCollector {
	labels := []string{"database", "backend_type", "object", "context"}
	counter := counterFactory(ioSubSystem, labels)
	gauge := gaugeFactory(ioSubSystem, labels)
	return &PgStatIOCollector{
		dbClients: dbClients,

		reads:         counter("reads_total", "Read operations issued"),
		readTime:      counter("read_time_seconds_total", "Total time spent in read operations, seconds"),
		writes:        counter("writes_total", "Write operations issued"),
		writeTime:     counter("write_time_seconds_total", "Total time spent in write operations, seconds"),
		writebacks:    counter("writebacks_total", "Writeback operations (dirty buffers flushed)"),
		writebackTime: counter("writeback_time_seconds_total", "Total time spent in writeback operations, seconds"),
		extends:       counter("extends_total", "Relation-extension operations"),
		extendTime:    counter("extend_time_seconds_total", "Total time spent extending relations, seconds"),
		opBytes:       gauge("op_bytes", "Block size for I/O operations in this bucket, bytes (typically 8192)"),
		hits:          counter("hits_total", "Shared-buffer hits"),
		evictions:     counter("evictions_total", "Buffers evicted to make room"),
		reuses:        counter("reuses_total", "Buffers reused from a previous strategy ring"),
		fsyncs:        counter("fsyncs_total", "fsync operations issued"),
		fsyncTime:     counter("fsync_time_seconds_total", "Total time spent in fsync operations, seconds"),
		statsReset:    gauge("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatIOCollector) Describe(ch chan<- *prometheus.Desc) {
	c.reads.Describe(ch)
	c.readTime.Describe(ch)
	c.writes.Describe(ch)
	c.writeTime.Describe(ch)
	c.writebacks.Describe(ch)
	c.writebackTime.Describe(ch)
	c.extends.Describe(ch)
	c.extendTime.Describe(ch)
	c.opBytes.Describe(ch)
	c.hits.Describe(ch)
	c.evictions.Describe(ch)
	c.reuses.Describe(ch)
	c.fsyncs.Describe(ch)
	c.fsyncTime.Describe(ch)
	c.statsReset.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatIOCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatIOCollector) collectInto(ch chan<- prometheus.Metric) {
	c.reads.Collect(ch)
	c.readTime.Collect(ch)
	c.writes.Collect(ch)
	c.writeTime.Collect(ch)
	c.writebacks.Collect(ch)
	c.writebackTime.Collect(ch)
	c.extends.Collect(ch)
	c.extendTime.Collect(ch)
	c.opBytes.Collect(ch)
	c.hits.Collect(ch)
	c.evictions.Collect(ch)
	c.reuses.Collect(ch)
	c.fsyncs.Collect(ch)
	c.fsyncTime.Collect(ch)
	c.statsReset.Collect(ch)
}

func (c *PgStatIOCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatIO(ctx)
	if err != nil {
		return fmt.Errorf("io stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatIOCollector) emit(stats []*model.PgStatIO) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.BackendType.String,
			stat.Object.String,
			stat.Context.String,
		}
		emitCounter := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), labels...)
			}
		}
		emitGauge := func(vec *prometheus.GaugeVec, v pgtype.Int8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(float64(v.Int64))
			}
		}
		emitMillisAsSecs := func(cd *counterDelta, v pgtype.Float8) {
			if v.Valid {
				cd.Observe(v.Float64/1000.0, labels...)
			}
		}
		emitTime := func(vec *prometheus.GaugeVec, v pgtype.Timestamptz) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(float64(v.Time.Unix()))
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
