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

	sampleBlksTotal   *prometheus.Desc
	sampleBlksScanned *prometheus.Desc
	extStatsTotal     *prometheus.Desc
	extStatsComputed  *prometheus.Desc
	childTablesTotal  *prometheus.Desc
	childTablesDone   *prometheus.Desc
}

// NewPgStatProgressAnalyzeCollector instantiates a new PgStatProgressAnalyzeCollector.
func NewPgStatProgressAnalyzeCollector(dbClients []*db.Client) *PgStatProgressAnalyzeCollector {
	labels := []string{"database", "datname", "relid", "phase"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, progressAnalyzeSubSystem, name), help, labels, nil)
	}
	return &PgStatProgressAnalyzeCollector{
		dbClients: dbClients,

		sampleBlksTotal:   desc("sample_blks_total", "Total heap blocks to sample"),
		sampleBlksScanned: desc("sample_blks_scanned", "Heap blocks scanned so far"),
		extStatsTotal:     desc("ext_stats_total", "Extended statistics to compute"),
		extStatsComputed:  desc("ext_stats_computed", "Extended statistics computed so far"),
		childTablesTotal:  desc("child_tables_total", "Child tables to analyze (partitioned)"),
		childTablesDone:   desc("child_tables_done", "Child tables analyzed so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressAnalyzeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.sampleBlksTotal
	ch <- c.sampleBlksScanned
	ch <- c.extStatsTotal
	ch <- c.extStatsComputed
	ch <- c.childTablesTotal
	ch <- c.childTablesDone
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
			c.emit(stats, ch)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatProgressAnalyzeCollector) emit(stats []*model.PgStatProgressAnalyze, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.DatName.String, stat.RelID.String, stat.Phase.String}
		emitInt(ch, c.sampleBlksTotal, stat.SampleBlksTotal, labels)
		emitInt(ch, c.sampleBlksScanned, stat.SampleBlksScanned, labels)
		emitInt(ch, c.extStatsTotal, stat.ExtStatsTotal, labels)
		emitInt(ch, c.extStatsComputed, stat.ExtStatsComputed, labels)
		emitInt(ch, c.childTablesTotal, stat.ChildTablesTotal, labels)
		emitInt(ch, c.childTablesDone, stat.ChildTablesDone, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_basebackup (PG 13+)
// ---------------------------------------------------------------------------

// PgStatProgressBasebackupCollector emits live pg_basebackup progress.
type PgStatProgressBasebackupCollector struct {
	dbClients []*db.Client

	backupTotalBytes    *prometheus.Desc
	backupStreamedBytes *prometheus.Desc
	tablespacesTotal    *prometheus.Desc
	tablespacesStreamed *prometheus.Desc
}

// NewPgStatProgressBasebackupCollector instantiates a new PgStatProgressBasebackupCollector.
func NewPgStatProgressBasebackupCollector(dbClients []*db.Client) *PgStatProgressBasebackupCollector {
	labels := []string{"database", "phase"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, progressBasebackupSubSystem, name), help, labels, nil)
	}
	return &PgStatProgressBasebackupCollector{
		dbClients: dbClients,

		backupTotalBytes:    desc("backup_total_bytes", "Estimated total backup size, bytes"),
		backupStreamedBytes: desc("backup_streamed_bytes", "Bytes streamed so far"),
		tablespacesTotal:    desc("tablespaces_total", "Tablespaces to stream"),
		tablespacesStreamed: desc("tablespaces_streamed", "Tablespaces streamed so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressBasebackupCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.backupTotalBytes
	ch <- c.backupStreamedBytes
	ch <- c.tablespacesTotal
	ch <- c.tablespacesStreamed
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
			c.emit(stats, ch)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatProgressBasebackupCollector) emit(stats []*model.PgStatProgressBasebackup, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.Phase.String}
		emitInt(ch, c.backupTotalBytes, stat.BackupTotalBytes, labels)
		emitInt(ch, c.backupStreamedBytes, stat.BackupStreamedBytes, labels)
		emitInt(ch, c.tablespacesTotal, stat.TablespacesTotal, labels)
		emitInt(ch, c.tablespacesStreamed, stat.TablespacesStreamed, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_copy (PG 14+)
// ---------------------------------------------------------------------------

// PgStatProgressCopyCollector emits live COPY progress.
type PgStatProgressCopyCollector struct {
	dbClients []*db.Client

	bytesTotal      *prometheus.Desc
	bytesProcessed  *prometheus.Desc
	tuplesProcessed *prometheus.Desc
	tuplesExcluded  *prometheus.Desc
}

// NewPgStatProgressCopyCollector instantiates a new PgStatProgressCopyCollector.
func NewPgStatProgressCopyCollector(dbClients []*db.Client) *PgStatProgressCopyCollector {
	labels := []string{"database", "datname", "relid", "command", "type"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, progressCopySubSystem, name), help, labels, nil)
	}
	return &PgStatProgressCopyCollector{
		dbClients: dbClients,

		bytesTotal:      desc("bytes_total", "Estimated total bytes to process (0 if unknown)"),
		bytesProcessed:  desc("bytes_processed", "Bytes processed so far"),
		tuplesProcessed: desc("tuples_processed", "Tuples processed so far"),
		tuplesExcluded:  desc("tuples_excluded", "Tuples excluded by WHERE clause"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressCopyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.bytesTotal
	ch <- c.bytesProcessed
	ch <- c.tuplesProcessed
	ch <- c.tuplesExcluded
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
			c.emit(stats, ch)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatProgressCopyCollector) emit(stats []*model.PgStatProgressCopy, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.DatName.String, stat.RelID.String, stat.Command.String, stat.Type.String}
		emitInt(ch, c.bytesTotal, stat.BytesTotal, labels)
		emitInt(ch, c.bytesProcessed, stat.BytesProcessed, labels)
		emitInt(ch, c.tuplesProcessed, stat.TuplesProcessed, labels)
		emitInt(ch, c.tuplesExcluded, stat.TuplesExcluded, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_create_index
// ---------------------------------------------------------------------------

// PgStatProgressCreateIndexCollector emits live CREATE INDEX / REINDEX progress.
type PgStatProgressCreateIndexCollector struct {
	dbClients []*db.Client

	lockersTotal    *prometheus.Desc
	lockersDone     *prometheus.Desc
	blocksTotal     *prometheus.Desc
	blocksDone      *prometheus.Desc
	tuplesTotal     *prometheus.Desc
	tuplesDone      *prometheus.Desc
	partitionsTotal *prometheus.Desc
	partitionsDone  *prometheus.Desc
}

// NewPgStatProgressCreateIndexCollector instantiates a new PgStatProgressCreateIndexCollector.
func NewPgStatProgressCreateIndexCollector(dbClients []*db.Client) *PgStatProgressCreateIndexCollector {
	labels := []string{"database", "datname", "relid", "index_relid", "command", "phase"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, progressCreateIndexSubSystem, name), help, labels, nil)
	}
	return &PgStatProgressCreateIndexCollector{
		dbClients: dbClients,

		lockersTotal:    desc("lockers_total", "Transactions holding conflicting locks"),
		lockersDone:     desc("lockers_done", "Locking transactions that have finished"),
		blocksTotal:     desc("blocks_total", "Blocks to process in this phase"),
		blocksDone:      desc("blocks_done", "Blocks processed so far"),
		tuplesTotal:     desc("tuples_total", "Tuples to process in this phase"),
		tuplesDone:      desc("tuples_done", "Tuples processed so far"),
		partitionsTotal: desc("partitions_total", "Child partitions to process"),
		partitionsDone:  desc("partitions_done", "Child partitions processed so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressCreateIndexCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.lockersTotal
	ch <- c.lockersDone
	ch <- c.blocksTotal
	ch <- c.blocksDone
	ch <- c.tuplesTotal
	ch <- c.tuplesDone
	ch <- c.partitionsTotal
	ch <- c.partitionsDone
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
			c.emit(stats, ch)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatProgressCreateIndexCollector) emit(stats []*model.PgStatProgressCreateIndex, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String, stat.DatName.String, stat.RelID.String,
			stat.IndexRelID.String, stat.Command.String, stat.Phase.String,
		}
		emitInt(ch, c.lockersTotal, stat.LockersTotal, labels)
		emitInt(ch, c.lockersDone, stat.LockersDone, labels)
		emitInt(ch, c.blocksTotal, stat.BlocksTotal, labels)
		emitInt(ch, c.blocksDone, stat.BlocksDone, labels)
		emitInt(ch, c.tuplesTotal, stat.TuplesTotal, labels)
		emitInt(ch, c.tuplesDone, stat.TuplesDone, labels)
		emitInt(ch, c.partitionsTotal, stat.PartitionsTotal, labels)
		emitInt(ch, c.partitionsDone, stat.PartitionsDone, labels)
	}
}

// ---------------------------------------------------------------------------
// pg_stat_progress_cluster
// ---------------------------------------------------------------------------

// PgStatProgressClusterCollector emits live CLUSTER / VACUUM FULL progress.
type PgStatProgressClusterCollector struct {
	dbClients []*db.Client

	heapTuplesScanned *prometheus.Desc
	heapTuplesWritten *prometheus.Desc
	heapBlksTotal     *prometheus.Desc
	heapBlksScanned   *prometheus.Desc
	indexRebuildCount *prometheus.Desc
}

// NewPgStatProgressClusterCollector instantiates a new PgStatProgressClusterCollector.
func NewPgStatProgressClusterCollector(dbClients []*db.Client) *PgStatProgressClusterCollector {
	labels := []string{"database", "datname", "relid", "command", "phase"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, progressClusterSubSystem, name), help, labels, nil)
	}
	return &PgStatProgressClusterCollector{
		dbClients: dbClients,

		heapTuplesScanned: desc("heap_tuples_scanned", "Heap tuples scanned so far"),
		heapTuplesWritten: desc("heap_tuples_written", "Heap tuples written so far"),
		heapBlksTotal:     desc("heap_blks_total", "Total heap blocks in this relation"),
		heapBlksScanned:   desc("heap_blks_scanned", "Heap blocks scanned so far (sequential phase)"),
		indexRebuildCount: desc("index_rebuild_count", "Indexes rebuilt so far"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressClusterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.heapTuplesScanned
	ch <- c.heapTuplesWritten
	ch <- c.heapBlksTotal
	ch <- c.heapBlksScanned
	ch <- c.indexRebuildCount
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
			c.emit(stats, ch)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatProgressClusterCollector) emit(stats []*model.PgStatProgressCluster, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.DatName.String, stat.RelID.String, stat.Command.String, stat.Phase.String}
		emitInt(ch, c.heapTuplesScanned, stat.HeapTuplesScanned, labels)
		emitInt(ch, c.heapTuplesWritten, stat.HeapTuplesWritten, labels)
		emitInt(ch, c.heapBlksTotal, stat.HeapBlksTotal, labels)
		emitInt(ch, c.heapBlksScanned, stat.HeapBlksScanned, labels)
		emitInt(ch, c.indexRebuildCount, stat.IndexRebuildCount, labels)
	}
}

// emitInt is the shared gauge-emit helper for progress collectors.
func emitInt(ch chan<- prometheus.Metric, desc *prometheus.Desc, v pgtype.Int8, labels []string) {
	if v.Valid {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Int64), labels...)
	}
}
