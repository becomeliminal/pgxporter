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

// All the pg_stat_progress_* collectors follow the same pattern: emit
// simple Int8 gauges per row, with a small set of enum labels. Pre-14
// gating (or pre-13 for analyze/basebackup) is handled in the db layer
// — each Select returns nil on unsupported servers so no metrics emit.

// ---------------------------------------------------------------------------
// pg_stat_progress_analyze (PG 13+)
// ---------------------------------------------------------------------------

// PgStatProgressAnalyzeCollector emits live ANALYZE progress.
type PgStatProgressAnalyzeCollector struct {
	dbClients []*db.Client

	sampleBlksTotal   *prometheus.GaugeVec
	sampleBlksScanned *prometheus.GaugeVec
	extStatsTotal     *prometheus.GaugeVec
	extStatsComputed  *prometheus.GaugeVec
	childTablesTotal  *prometheus.GaugeVec
	childTablesDone   *prometheus.GaugeVec
}

// NewPgStatProgressAnalyzeCollector instantiates a new PgStatProgressAnalyzeCollector.
func NewPgStatProgressAnalyzeCollector(dbClients []*db.Client) *PgStatProgressAnalyzeCollector {
	labels := []string{"database", "datname", "relid", "phase"}
	gauge := gaugeFactory(progressAnalyzeSubSystem, labels)
	return &PgStatProgressAnalyzeCollector{
		dbClients: dbClients,

		sampleBlksTotal:   gauge("sample_blks_total", "Total heap blocks to sample"),
		sampleBlksScanned: gauge("sample_blks_scanned", "Heap blocks scanned so far"),
		extStatsTotal:     gauge("ext_stats_total", "Extended statistics to compute"),
		extStatsComputed:  gauge("ext_stats_computed", "Extended statistics computed so far"),
		childTablesTotal:  gauge("child_tables_total", "Child tables to analyze (partitioned)"),
		childTablesDone:   gauge("child_tables_done", "Child tables analyzed so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressAnalyzeCollector) Describe(ch chan<- *prometheus.Desc) {
	c.sampleBlksTotal.Describe(ch)
	c.sampleBlksScanned.Describe(ch)
	c.extStatsTotal.Describe(ch)
	c.extStatsComputed.Describe(ch)
	c.childTablesTotal.Describe(ch)
	c.childTablesDone.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatProgressAnalyzeCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error {
			stats, err := dbClient.SelectPgStatProgressAnalyze(gctx)
			if err != nil {
				return fmt.Errorf("progress_analyze: %w", err)
			}
			c.emit(stats)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatProgressAnalyzeCollector) collectInto(ch chan<- prometheus.Metric) {
	c.sampleBlksTotal.Collect(ch)
	c.sampleBlksScanned.Collect(ch)
	c.extStatsTotal.Collect(ch)
	c.extStatsComputed.Collect(ch)
	c.childTablesTotal.Collect(ch)
	c.childTablesDone.Collect(ch)
}

func (c *PgStatProgressAnalyzeCollector) emit(stats []*model.PgStatProgressAnalyze) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.DatName.String, stat.RelID.String, stat.Phase.String}
		setInt(c.sampleBlksTotal, stat.SampleBlksTotal, labels)
		setInt(c.sampleBlksScanned, stat.SampleBlksScanned, labels)
		setInt(c.extStatsTotal, stat.ExtStatsTotal, labels)
		setInt(c.extStatsComputed, stat.ExtStatsComputed, labels)
		setInt(c.childTablesTotal, stat.ChildTablesTotal, labels)
		setInt(c.childTablesDone, stat.ChildTablesDone, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_basebackup (PG 13+)
// ---------------------------------------------------------------------------

// PgStatProgressBasebackupCollector emits live pg_basebackup progress.
type PgStatProgressBasebackupCollector struct {
	dbClients []*db.Client

	backupTotalBytes    *prometheus.GaugeVec
	backupStreamedBytes *prometheus.GaugeVec
	tablespacesTotal    *prometheus.GaugeVec
	tablespacesStreamed *prometheus.GaugeVec
}

// NewPgStatProgressBasebackupCollector instantiates a new PgStatProgressBasebackupCollector.
func NewPgStatProgressBasebackupCollector(dbClients []*db.Client) *PgStatProgressBasebackupCollector {
	labels := []string{"database", "phase"}
	gauge := gaugeFactory(progressBasebackupSubSystem, labels)
	return &PgStatProgressBasebackupCollector{
		dbClients: dbClients,

		backupTotalBytes:    gauge("backup_total_bytes", "Estimated total backup size, bytes"),
		backupStreamedBytes: gauge("backup_streamed_bytes", "Bytes streamed so far"),
		tablespacesTotal:    gauge("tablespaces_total", "Tablespaces to stream"),
		tablespacesStreamed: gauge("tablespaces_streamed", "Tablespaces streamed so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressBasebackupCollector) Describe(ch chan<- *prometheus.Desc) {
	c.backupTotalBytes.Describe(ch)
	c.backupStreamedBytes.Describe(ch)
	c.tablespacesTotal.Describe(ch)
	c.tablespacesStreamed.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatProgressBasebackupCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error {
			stats, err := dbClient.SelectPgStatProgressBasebackup(gctx)
			if err != nil {
				return fmt.Errorf("progress_basebackup: %w", err)
			}
			c.emit(stats)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatProgressBasebackupCollector) collectInto(ch chan<- prometheus.Metric) {
	c.backupTotalBytes.Collect(ch)
	c.backupStreamedBytes.Collect(ch)
	c.tablespacesTotal.Collect(ch)
	c.tablespacesStreamed.Collect(ch)
}

func (c *PgStatProgressBasebackupCollector) emit(stats []*model.PgStatProgressBasebackup) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.Phase.String}
		setInt(c.backupTotalBytes, stat.BackupTotalBytes, labels)
		setInt(c.backupStreamedBytes, stat.BackupStreamedBytes, labels)
		setInt(c.tablespacesTotal, stat.TablespacesTotal, labels)
		setInt(c.tablespacesStreamed, stat.TablespacesStreamed, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_copy (PG 14+)
// ---------------------------------------------------------------------------

// PgStatProgressCopyCollector emits live COPY progress.
type PgStatProgressCopyCollector struct {
	dbClients []*db.Client

	bytesTotal      *prometheus.GaugeVec
	bytesProcessed  *prometheus.GaugeVec
	tuplesProcessed *prometheus.GaugeVec
	tuplesExcluded  *prometheus.GaugeVec
}

// NewPgStatProgressCopyCollector instantiates a new PgStatProgressCopyCollector.
func NewPgStatProgressCopyCollector(dbClients []*db.Client) *PgStatProgressCopyCollector {
	labels := []string{"database", "datname", "relid", "command", "type"}
	gauge := gaugeFactory(progressCopySubSystem, labels)
	return &PgStatProgressCopyCollector{
		dbClients: dbClients,

		bytesTotal:      gauge("bytes_total", "Estimated total bytes to process (0 if unknown)"),
		bytesProcessed:  gauge("bytes_processed", "Bytes processed so far"),
		tuplesProcessed: gauge("tuples_processed", "Tuples processed so far"),
		tuplesExcluded:  gauge("tuples_excluded", "Tuples excluded by WHERE clause"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressCopyCollector) Describe(ch chan<- *prometheus.Desc) {
	c.bytesTotal.Describe(ch)
	c.bytesProcessed.Describe(ch)
	c.tuplesProcessed.Describe(ch)
	c.tuplesExcluded.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatProgressCopyCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error {
			stats, err := dbClient.SelectPgStatProgressCopy(gctx)
			if err != nil {
				return fmt.Errorf("progress_copy: %w", err)
			}
			c.emit(stats)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatProgressCopyCollector) collectInto(ch chan<- prometheus.Metric) {
	c.bytesTotal.Collect(ch)
	c.bytesProcessed.Collect(ch)
	c.tuplesProcessed.Collect(ch)
	c.tuplesExcluded.Collect(ch)
}

func (c *PgStatProgressCopyCollector) emit(stats []*model.PgStatProgressCopy) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.DatName.String, stat.RelID.String, stat.Command.String, stat.Type.String}
		setInt(c.bytesTotal, stat.BytesTotal, labels)
		setInt(c.bytesProcessed, stat.BytesProcessed, labels)
		setInt(c.tuplesProcessed, stat.TuplesProcessed, labels)
		setInt(c.tuplesExcluded, stat.TuplesExcluded, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_create_index
// ---------------------------------------------------------------------------

// PgStatProgressCreateIndexCollector emits live CREATE INDEX / REINDEX progress.
type PgStatProgressCreateIndexCollector struct {
	dbClients []*db.Client

	lockersTotal    *prometheus.GaugeVec
	lockersDone     *prometheus.GaugeVec
	blocksTotal     *prometheus.GaugeVec
	blocksDone      *prometheus.GaugeVec
	tuplesTotal     *prometheus.GaugeVec
	tuplesDone      *prometheus.GaugeVec
	partitionsTotal *prometheus.GaugeVec
	partitionsDone  *prometheus.GaugeVec
}

// NewPgStatProgressCreateIndexCollector instantiates a new PgStatProgressCreateIndexCollector.
func NewPgStatProgressCreateIndexCollector(dbClients []*db.Client) *PgStatProgressCreateIndexCollector {
	labels := []string{"database", "datname", "relid", "index_relid", "command", "phase"}
	gauge := gaugeFactory(progressCreateIndexSubSystem, labels)
	return &PgStatProgressCreateIndexCollector{
		dbClients: dbClients,

		lockersTotal:    gauge("lockers_total", "Transactions holding conflicting locks"),
		lockersDone:     gauge("lockers_done", "Locking transactions that have finished"),
		blocksTotal:     gauge("blocks_total", "Blocks to process in this phase"),
		blocksDone:      gauge("blocks_done", "Blocks processed so far"),
		tuplesTotal:     gauge("tuples_total", "Tuples to process in this phase"),
		tuplesDone:      gauge("tuples_done", "Tuples processed so far"),
		partitionsTotal: gauge("partitions_total", "Child partitions to process"),
		partitionsDone:  gauge("partitions_done", "Child partitions processed so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressCreateIndexCollector) Describe(ch chan<- *prometheus.Desc) {
	c.lockersTotal.Describe(ch)
	c.lockersDone.Describe(ch)
	c.blocksTotal.Describe(ch)
	c.blocksDone.Describe(ch)
	c.tuplesTotal.Describe(ch)
	c.tuplesDone.Describe(ch)
	c.partitionsTotal.Describe(ch)
	c.partitionsDone.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatProgressCreateIndexCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error {
			stats, err := dbClient.SelectPgStatProgressCreateIndex(gctx)
			if err != nil {
				return fmt.Errorf("progress_create_index: %w", err)
			}
			c.emit(stats)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatProgressCreateIndexCollector) collectInto(ch chan<- prometheus.Metric) {
	c.lockersTotal.Collect(ch)
	c.lockersDone.Collect(ch)
	c.blocksTotal.Collect(ch)
	c.blocksDone.Collect(ch)
	c.tuplesTotal.Collect(ch)
	c.tuplesDone.Collect(ch)
	c.partitionsTotal.Collect(ch)
	c.partitionsDone.Collect(ch)
}

func (c *PgStatProgressCreateIndexCollector) emit(stats []*model.PgStatProgressCreateIndex) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String, stat.DatName.String, stat.RelID.String,
			stat.IndexRelID.String, stat.Command.String, stat.Phase.String,
		}
		setInt(c.lockersTotal, stat.LockersTotal, labels)
		setInt(c.lockersDone, stat.LockersDone, labels)
		setInt(c.blocksTotal, stat.BlocksTotal, labels)
		setInt(c.blocksDone, stat.BlocksDone, labels)
		setInt(c.tuplesTotal, stat.TuplesTotal, labels)
		setInt(c.tuplesDone, stat.TuplesDone, labels)
		setInt(c.partitionsTotal, stat.PartitionsTotal, labels)
		setInt(c.partitionsDone, stat.PartitionsDone, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_cluster
// ---------------------------------------------------------------------------

// PgStatProgressClusterCollector emits live CLUSTER / VACUUM FULL progress.
type PgStatProgressClusterCollector struct {
	dbClients []*db.Client

	heapTuplesScanned *prometheus.GaugeVec
	heapTuplesWritten *prometheus.GaugeVec
	heapBlksTotal     *prometheus.GaugeVec
	heapBlksScanned   *prometheus.GaugeVec
	indexRebuildCount *prometheus.GaugeVec
}

// NewPgStatProgressClusterCollector instantiates a new PgStatProgressClusterCollector.
func NewPgStatProgressClusterCollector(dbClients []*db.Client) *PgStatProgressClusterCollector {
	labels := []string{"database", "datname", "relid", "command", "phase"}
	gauge := gaugeFactory(progressClusterSubSystem, labels)
	return &PgStatProgressClusterCollector{
		dbClients: dbClients,

		heapTuplesScanned: gauge("heap_tuples_scanned", "Heap tuples scanned so far"),
		heapTuplesWritten: gauge("heap_tuples_written", "Heap tuples written so far"),
		heapBlksTotal:     gauge("heap_blks_total", "Total heap blocks in this relation"),
		heapBlksScanned:   gauge("heap_blks_scanned", "Heap blocks scanned so far (sequential phase)"),
		indexRebuildCount: gauge("index_rebuild_count", "Indexes rebuilt so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressClusterCollector) Describe(ch chan<- *prometheus.Desc) {
	c.heapTuplesScanned.Describe(ch)
	c.heapTuplesWritten.Describe(ch)
	c.heapBlksTotal.Describe(ch)
	c.heapBlksScanned.Describe(ch)
	c.indexRebuildCount.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatProgressClusterCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error {
			stats, err := dbClient.SelectPgStatProgressCluster(gctx)
			if err != nil {
				return fmt.Errorf("progress_cluster: %w", err)
			}
			c.emit(stats)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatProgressClusterCollector) collectInto(ch chan<- prometheus.Metric) {
	c.heapTuplesScanned.Collect(ch)
	c.heapTuplesWritten.Collect(ch)
	c.heapBlksTotal.Collect(ch)
	c.heapBlksScanned.Collect(ch)
	c.indexRebuildCount.Collect(ch)
}

func (c *PgStatProgressClusterCollector) emit(stats []*model.PgStatProgressCluster) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.DatName.String, stat.RelID.String, stat.Command.String, stat.Phase.String}
		setInt(c.heapTuplesScanned, stat.HeapTuplesScanned, labels)
		setInt(c.heapTuplesWritten, stat.HeapTuplesWritten, labels)
		setInt(c.heapBlksTotal, stat.HeapBlksTotal, labels)
		setInt(c.heapBlksScanned, stat.HeapBlksScanned, labels)
		setInt(c.indexRebuildCount, stat.IndexRebuildCount, labels)
	}
}

// setInt is the shared gauge-set helper for progress collectors.
func setInt(vec *prometheus.GaugeVec, v pgtype.Int8, labels []string) {
	if v.Valid {
		vec.WithLabelValues(labels...).Set(float64(v.Int64))
	}
}
