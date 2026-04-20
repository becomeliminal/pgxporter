package collectors

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatUserTableCollector collects from pg_stat_user_tables.
type PgStatUserTableCollector struct {
	dbClients []*db.Client
	mutex     sync.RWMutex

	seqScan          *counterDelta
	lastSeqScan      *prometheus.GaugeVec // PG 16+
	seqTupRead       *counterDelta
	idxScan          *counterDelta
	lastIdxScan      *prometheus.GaugeVec // PG 16+
	idxTupFetch      *counterDelta
	nTupIns          *counterDelta
	nTupUpd          *counterDelta
	nTupDel          *counterDelta
	nTupHotUpd       *counterDelta
	nTupNewpageUpd   *counterDelta // PG 17+
	nLiveTup         *prometheus.GaugeVec
	nDeadTup         *prometheus.GaugeVec
	nModSinceAnalyze *prometheus.GaugeVec
	nInsSinceVacuum  *prometheus.GaugeVec // PG 13+
	lastVacuum       *prometheus.GaugeVec
	lastAutoVacuum   *prometheus.GaugeVec
	lastAnalyze      *prometheus.GaugeVec
	lastAutoAnalyze  *prometheus.GaugeVec
	vacuumCount      *counterDelta
	autoVacuumCount  *counterDelta
	analyzeCount     *counterDelta
	autoAnalyzeCount *counterDelta
}

// NewPgStatUserTableCollector instantiates and returns a new PgStatUserTableCollector.
func NewPgStatUserTableCollector(dbClients []*db.Client) *PgStatUserTableCollector {
	variableLabels := []string{"database", "schemaname", "relname"}
	counter := counterFactory(userTablesSubSystem, variableLabels)
	gauge := gaugeFactory(userTablesSubSystem, variableLabels)
	return &PgStatUserTableCollector{
		dbClients:        dbClients,
		seqScan:          counter("sequential_scan", "Number of sequential scans initiated on this table"),
		seqTupRead:       counter("sequential_scan_tup_read", "Number of live rows fetched by sequential scans"),
		idxScan:          counter("index_scan", "Number of index scans initiated on this table"),
		idxTupFetch:      counter("index_tup_fetch", "Number of live rows fetched by index scans"),
		nTupIns:          counter("n_tup_ins", "Number of rows inserted"),
		nTupUpd:          counter("n_tup_upd", "Number of rows updated"),
		nTupDel:          counter("n_tup_del", "Number of rows deleted"),
		nTupHotUpd:       counter("n_tup_hot_upd", "Number of rows HOT updated"),
		nLiveTup:         gauge("n_live_tup", "Estimated number of live rows"),
		nDeadTup:         gauge("n_dead_tup", "Estimated number of dead rows"),
		nModSinceAnalyze: gauge("n_mod_since_analyze", "Estimated number of rows changed since last analyze"),
		lastVacuum:       gauge("last_vacuum", "Last time at which this table was manually vacuumed (not counting VACUUM FULL)"),
		lastAutoVacuum:   gauge("last_autovacuum", "Last time at which this table was vacuumed by the autovacuum daemon"),
		lastAnalyze:      gauge("last_analyze", "Last time at which this table was manually analyzed"),
		lastAutoAnalyze:  gauge("last_autoanalyze", "Last time at which this table was analyzed by the autovacuum daemon"),
		vacuumCount:      counter("vacuum_count", "Number of times this table has been manually vacuumed (not counting VACUUM FULL)"),
		autoVacuumCount:  counter("autovacuum_count", "Number of times this table has been vacuumed by the autovacuum daemon"),
		analyzeCount:     counter("analyze_count", "Number of times this table has been manually analyzed"),
		autoAnalyzeCount: counter("autoanalyze_count", "Number of times this table has been analyzed by the autovacuum daemon"),

		// Version-gated descriptors — emitted only when the connected server
		// is new enough. See [db.PgStatUserTable] struct comments.
		lastSeqScan:     gauge("last_seq_scan", "Time of the last sequential scan on this table, as unix microseconds (PG 16+)"),
		lastIdxScan:     gauge("last_idx_scan", "Time of the last index scan on this table, as unix microseconds (PG 16+)"),
		nTupNewpageUpd:  counter("n_tup_newpage_upd", "Number of rows updated via a new-page mechanism, i.e. not HOT (PG 17+)"),
		nInsSinceVacuum: gauge("n_ins_since_vacuum", "Estimated number of rows inserted since this table was last vacuumed (PG 13+)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatUserTableCollector) Describe(ch chan<- *prometheus.Desc) {
	c.seqScan.Describe(ch)
	c.lastSeqScan.Describe(ch)
	c.seqTupRead.Describe(ch)
	c.idxScan.Describe(ch)
	c.lastIdxScan.Describe(ch)
	c.idxTupFetch.Describe(ch)
	c.nTupIns.Describe(ch)
	c.nTupUpd.Describe(ch)
	c.nTupDel.Describe(ch)
	c.nTupHotUpd.Describe(ch)
	c.nTupNewpageUpd.Describe(ch)
	c.nLiveTup.Describe(ch)
	c.nDeadTup.Describe(ch)
	c.nModSinceAnalyze.Describe(ch)
	c.nInsSinceVacuum.Describe(ch)
	c.lastVacuum.Describe(ch)
	c.lastAutoVacuum.Describe(ch)
	c.lastAnalyze.Describe(ch)
	c.lastAutoAnalyze.Describe(ch)
	c.vacuumCount.Describe(ch)
	c.autoVacuumCount.Describe(ch)
	c.analyzeCount.Describe(ch)
	c.autoAnalyzeCount.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatUserTableCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatUserTableCollector) collectInto(ch chan<- prometheus.Metric) {
	c.seqScan.Collect(ch)
	c.lastSeqScan.Collect(ch)
	c.seqTupRead.Collect(ch)
	c.idxScan.Collect(ch)
	c.lastIdxScan.Collect(ch)
	c.idxTupFetch.Collect(ch)
	c.nTupIns.Collect(ch)
	c.nTupUpd.Collect(ch)
	c.nTupDel.Collect(ch)
	c.nTupHotUpd.Collect(ch)
	c.nTupNewpageUpd.Collect(ch)
	c.nLiveTup.Collect(ch)
	c.nDeadTup.Collect(ch)
	c.nModSinceAnalyze.Collect(ch)
	c.nInsSinceVacuum.Collect(ch)
	c.lastVacuum.Collect(ch)
	c.lastAutoVacuum.Collect(ch)
	c.lastAnalyze.Collect(ch)
	c.lastAutoAnalyze.Collect(ch)
	c.vacuumCount.Collect(ch)
	c.autoVacuumCount.Collect(ch)
	c.analyzeCount.Collect(ch)
	c.autoAnalyzeCount.Collect(ch)
}

func (c *PgStatUserTableCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	userTableStats, err := dbClient.SelectPgStatUserTables(ctx)
	if err != nil {
		return fmt.Errorf("user table stats: %w", err)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.emit(userTableStats)
	return nil
}

// emit turns scanned pg_stat_user_tables rows into metrics, skipping NULL
// counter or timestamp columns. Partitioned-table-parent rows from PG 17+
// return NULL counters — without the Valid gating below, scany would try
// to assign NULL into a plain int field and the whole scrape would fail
// (the pgtype-everywhere refactor exists specifically to make this safe).
//
// Separated from scrape for unit-test coverage.
func (c *PgStatUserTableCollector) emit(stats []*model.PgStatUserTable) {
	for _, stat := range stats {
		database, schemaname, relname := stat.Database.String, stat.SchemaName.String, stat.RelName.String
		emitCounter := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), database, schemaname, relname)
			}
		}
		emitGauge := func(vec *prometheus.GaugeVec, v pgtype.Int8) {
			if v.Valid {
				vec.WithLabelValues(database, schemaname, relname).Set(float64(v.Int64))
			}
		}
		emitTime := func(vec *prometheus.GaugeVec, v pgtype.Timestamptz) {
			if v.Valid {
				vec.WithLabelValues(database, schemaname, relname).Set(float64(v.Time.UnixMicro()))
			}
		}
		emitCounter(c.seqScan, stat.SeqScan)
		emitTime(c.lastSeqScan, stat.LastSeqScan) // PG 16+, invalid on older
		emitCounter(c.seqTupRead, stat.SeqTupRead)
		emitCounter(c.idxScan, stat.IndexScan)
		emitTime(c.lastIdxScan, stat.LastIdxScan) // PG 16+, invalid on older
		emitCounter(c.idxTupFetch, stat.IndexTupFetch)
		emitCounter(c.nTupIns, stat.NTupInsert)
		emitCounter(c.nTupUpd, stat.NTupUpdate)
		emitCounter(c.nTupDel, stat.NTupDelete)
		emitCounter(c.nTupHotUpd, stat.NTupHotUpdate)
		emitCounter(c.nTupNewpageUpd, stat.NTupNewpageUpdate) // PG 17+
		emitGauge(c.nLiveTup, stat.NLiveTup)
		emitGauge(c.nDeadTup, stat.NDeadTup)
		emitGauge(c.nModSinceAnalyze, stat.NModSinceAnalyze)
		emitGauge(c.nInsSinceVacuum, stat.NInsSinceVacuum) // PG 13+
		emitTime(c.lastVacuum, stat.LastVacuum)
		emitTime(c.lastAutoVacuum, stat.LastAutoVacuum)
		emitTime(c.lastAnalyze, stat.LastAnalyze)
		emitTime(c.lastAutoAnalyze, stat.LastAutoAnalyze)
		emitCounter(c.vacuumCount, stat.VacuumCount)
		emitCounter(c.autoVacuumCount, stat.AutoVacuumCount)
		emitCounter(c.analyzeCount, stat.AnalyzeCount)
		emitCounter(c.autoAnalyzeCount, stat.AutoAnalyzeCount)
	}
}
