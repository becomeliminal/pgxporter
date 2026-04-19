package collectors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
)

// PgStatUserTableCollector collects from pg_stat_user_tables.
type PgStatUserTableCollector struct {
	dbClients []*db.Client
	mutex     sync.RWMutex

	seqScan          *prometheus.Desc
	seqTupRead       *prometheus.Desc
	idxScan          *prometheus.Desc
	idxTupFetch      *prometheus.Desc
	nTupIns          *prometheus.Desc
	nTupUpd          *prometheus.Desc
	nTupDel          *prometheus.Desc
	nTupHotUpd       *prometheus.Desc
	nLiveTup         *prometheus.Desc
	nDeadTup         *prometheus.Desc
	nModSinceAnalyze *prometheus.Desc
	lastVacuum       *prometheus.Desc
	lastAutoVacuum   *prometheus.Desc
	lastAnalyze      *prometheus.Desc
	lastAutoAnalyze  *prometheus.Desc
	vacuumCount      *prometheus.Desc
	autoVacuumCount  *prometheus.Desc
	analyzeCount     *prometheus.Desc
	autoAnalyzeCount *prometheus.Desc
}

// NewPgStatUserTableCollector instantiates and returns a new PgStatUserTableCollector.
func NewPgStatUserTableCollector(dbClients []*db.Client) *PgStatUserTableCollector {
	variableLabels := []string{"database", "schemaname", "relname"}
	return &PgStatUserTableCollector{
		dbClients: dbClients,
		seqScan: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "sequential_scan"),
			"Number of sequential scans initiated on this table",
			variableLabels,
			nil,
		),
		seqTupRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "sequential_scan_tup_read"),
			"Number of live rows fetched by sequential scans",
			variableLabels,
			nil,
		),
		idxScan: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "index_scan"),
			"Number of index scans initiated on this table",
			variableLabels,
			nil,
		),
		idxTupFetch: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "index_tup_fetch"),
			"Number of live rows fetched by index scans",
			variableLabels,
			nil,
		),
		nTupIns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "n_tup_ins"),
			"Number of rows inserted",
			variableLabels,
			nil,
		),
		nTupUpd: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "n_tup_upd"),
			"Number of rows updated",
			variableLabels,
			nil,
		),
		nTupDel: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "n_tup_del"),
			"Number of rows deleted",
			variableLabels,
			nil,
		),
		nTupHotUpd: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "n_tup_hot_upd"),
			"Number of rows HOT updated",
			variableLabels,
			nil,
		),
		nLiveTup: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "n_live_tup"),
			"Estimated number of live rows",
			variableLabels,
			nil,
		),
		nDeadTup: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "n_dead_tup"),
			"Estimated number of dead rows",
			variableLabels,
			nil,
		),
		nModSinceAnalyze: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "n_mod_since_analyze"),
			"Estimated number of rows changed since last analyze",
			variableLabels,
			nil,
		),
		lastVacuum: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "last_vacuum"),
			"Last time at which this table was manually vacuumed (not counting VACUUM FULL)",
			variableLabels,
			nil,
		),
		lastAutoVacuum: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "last_autovacuum"),
			"Last time at which this table was vacuumed by the autovacuum daemon",
			variableLabels,
			nil,
		),
		lastAnalyze: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "last_analyze"),
			"Last time at which this table was manually analyzed",
			variableLabels,
			nil,
		),
		lastAutoAnalyze: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "last_autoanalyze"),
			"Last time at which this table was analyzed by the autovacuum daemon",
			variableLabels,
			nil,
		),
		vacuumCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "vacuum_count"),
			"Number of times this table has been manually vacuumed (not counting VACUUM FULL)",
			variableLabels,
			nil,
		),
		autoVacuumCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "autovacuum_count"),
			"Number of times this table has been vacuumed by the autovacuum daemon",
			variableLabels,
			nil,
		),
		analyzeCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "analyze_count"),
			"Number of times this table has been manually analyzed",
			variableLabels,
			nil,
		),
		autoAnalyzeCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTablesSubSystem, "autoanalyze_count"),
			"Number of times this table has been analyzed by the autovacuum daemon",
			variableLabels,
			nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatUserTableCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.seqScan
	ch <- c.seqTupRead
	ch <- c.idxScan
	ch <- c.idxTupFetch
	ch <- c.nTupIns
	ch <- c.nTupUpd
	ch <- c.nTupDel
	ch <- c.nTupHotUpd
	ch <- c.nLiveTup
	ch <- c.nDeadTup
	ch <- c.nModSinceAnalyze
	ch <- c.lastVacuum
	ch <- c.lastAutoVacuum
	ch <- c.lastAnalyze
	ch <- c.lastAutoAnalyze
	ch <- c.vacuumCount
	ch <- c.autoVacuumCount
	ch <- c.analyzeCount
	ch <- c.autoAnalyzeCount
}

// Scrape implements our Scraper interface.
func (c *PgStatUserTableCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("user table scrape took %dms", time.Now().Sub(start).Milliseconds())
	}()
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

func (c *PgStatUserTableCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	userTableStats, err := dbClient.SelectPgStatUserTables(ctx)
	if err != nil {
		return fmt.Errorf("user table stats: %w", err)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, stat := range userTableStats {
		database, schemaname, relname := stat.Database.String, stat.SchemaName.String, stat.RelName.String
		emitInt := func(desc *prometheus.Desc, valueType prometheus.ValueType, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, valueType, float64(v.Int64), database, schemaname, relname)
			}
		}
		emitTime := func(desc *prometheus.Desc, v pgtype.Timestamptz) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Time.UnixMicro()), database, schemaname, relname)
			}
		}
		emitInt(c.seqScan, prometheus.CounterValue, stat.SeqScan)
		emitInt(c.seqTupRead, prometheus.CounterValue, stat.SeqTupRead)
		emitInt(c.idxScan, prometheus.CounterValue, stat.IndexScan)
		emitInt(c.idxTupFetch, prometheus.CounterValue, stat.IndexTupFetch)
		emitInt(c.nTupIns, prometheus.CounterValue, stat.NTupInsert)
		emitInt(c.nTupUpd, prometheus.CounterValue, stat.NTupUpdate)
		emitInt(c.nTupDel, prometheus.CounterValue, stat.NTupDelete)
		emitInt(c.nTupHotUpd, prometheus.CounterValue, stat.NTupHotUpdate)
		emitInt(c.nLiveTup, prometheus.GaugeValue, stat.NLiveTup)
		emitInt(c.nDeadTup, prometheus.GaugeValue, stat.NDeadTup)
		emitInt(c.nModSinceAnalyze, prometheus.GaugeValue, stat.NModSinceAnalyze)
		emitTime(c.lastVacuum, stat.LastVacuum)
		emitTime(c.lastAutoVacuum, stat.LastAutoVacuum)
		emitTime(c.lastAnalyze, stat.LastAnalyze)
		emitTime(c.lastAutoAnalyze, stat.LastAutoAnalyze)
		emitInt(c.vacuumCount, prometheus.CounterValue, stat.VacuumCount)
		emitInt(c.autoVacuumCount, prometheus.CounterValue, stat.AutoVacuumCount)
		emitInt(c.analyzeCount, prometheus.CounterValue, stat.AnalyzeCount)
		emitInt(c.autoAnalyzeCount, prometheus.CounterValue, stat.AutoAnalyzeCount)
	}
	return nil
}
