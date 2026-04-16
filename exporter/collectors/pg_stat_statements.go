package collectors

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

// PgStatStatementsCollector collects from pg_stat_statements.
type PgStatStatementsCollector struct {
	dbClients []*db.Client
	mutex     sync.RWMutex

	calls               *prometheus.Desc
	totalTimeSeconds    *prometheus.Desc
	minTimeSeconds      *prometheus.Desc
	maxTimeSeconds      *prometheus.Desc
	meanTimeSeconds     *prometheus.Desc
	stdDevTimeSeconds   *prometheus.Desc
	rows                *prometheus.Desc
	sharedBlksHit       *prometheus.Desc
	sharedBlksRead      *prometheus.Desc
	sharedBlksDirtied   *prometheus.Desc
	sharedBlksWritten   *prometheus.Desc
	localBlksHit        *prometheus.Desc
	localBlksRead       *prometheus.Desc
	localBlksDirtied    *prometheus.Desc
	localBlksWritten    *prometheus.Desc
	tempBlksRead        *prometheus.Desc
	tempBlksWritten     *prometheus.Desc
	blkReadTimeSeconds  *prometheus.Desc
	blkWriteTimeSeconds *prometheus.Desc
}

// NewPgStatStatementsCollector instantiates and returns a new PgStatStatementsCollector.
func NewPgStatStatementsCollector(dbClients []*db.Client) *PgStatStatementsCollector {
	variableLabels := []string{"database", "rolname", "datname", "queryid", "query"}
	return &PgStatStatementsCollector{
		dbClients: dbClients,

		calls: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "calls"),
			"",
			variableLabels,
			nil,
		),
		totalTimeSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "total_time_seconds"),
			"",
			variableLabels,
			nil,
		),
		minTimeSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "min_time_seconds"),
			"",
			variableLabels,
			nil,
		),
		maxTimeSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "max_time_seconds"),
			"",
			variableLabels,
			nil,
		),
		meanTimeSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "mean_time_seconds"),
			"",
			variableLabels,
			nil,
		),
		stdDevTimeSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "std_dev_time_seconds"),
			"",
			variableLabels,
			nil,
		),
		rows: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "rows"),
			"",
			variableLabels,
			nil,
		),
		sharedBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "shared_blks_hit"),
			"",
			variableLabels,
			nil,
		),
		sharedBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "shared_blks_read"),
			"",
			variableLabels,
			nil,
		),
		sharedBlksDirtied: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "shared_blks_dirtied"),
			"",
			variableLabels,
			nil,
		),
		sharedBlksWritten: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "shared_blks_written"),
			"",
			variableLabels,
			nil,
		),
		localBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "local_blks_hit"),
			"",
			variableLabels,
			nil,
		),
		localBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "local_blks_read"),
			"",
			variableLabels,
			nil,
		),
		localBlksDirtied: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "local_blks_dirtied"),
			"",
			variableLabels,
			nil,
		),
		localBlksWritten: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "local_blks_written"),
			"",
			variableLabels,
			nil,
		),
		tempBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "templ_blks_read"),
			"",
			variableLabels,
			nil,
		),
		tempBlksWritten: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "temp_blks_written"),
			"",
			variableLabels,
			nil,
		),
		blkReadTimeSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "blk_read_time_seconds"),
			"",
			variableLabels,
			nil,
		),
		blkWriteTimeSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statementsSubSystem, "blk_write_time_seconds"),
			"",
			variableLabels,
			nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatStatementsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.calls
	ch <- c.totalTimeSeconds
	ch <- c.minTimeSeconds
	ch <- c.maxTimeSeconds
	ch <- c.meanTimeSeconds
	ch <- c.stdDevTimeSeconds
	ch <- c.rows
	ch <- c.sharedBlksHit
	ch <- c.sharedBlksRead
	ch <- c.sharedBlksDirtied
	ch <- c.sharedBlksWritten
	ch <- c.localBlksHit
	ch <- c.localBlksRead
	ch <- c.localBlksDirtied
	ch <- c.localBlksWritten
	ch <- c.tempBlksRead
	ch <- c.tempBlksWritten
	ch <- c.blkReadTimeSeconds
	ch <- c.blkWriteTimeSeconds
}

// Collect implements the promtheus.Collector.
func (c *PgStatStatementsCollector) Collect(ch chan<- prometheus.Metric) {
	_ = c.Scrape(ch)
}

// Scrape implements our Scraper interfacc.
func (c *PgStatStatementsCollector) Scrape(ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("statement scrape took %dms", time.Now().Sub(start).Milliseconds())
	}()
	group := errgroup.Group{}
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(dbClient, ch) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatStatementsCollector) scrape(dbClient *db.Client, ch chan<- prometheus.Metric) error {
	statementStats, err := dbClient.SelectPgStatStatements(context.Background())
	if err != nil {
		return fmt.Errorf("statement stats: %w", err)
	}
	start := time.Now()
	log.Infof("statements lock aquire %dms", time.Now().Sub(start).Milliseconds())
	for _, stat := range statementStats {
		ch <- prometheus.MustNewConstMetric(c.calls, prometheus.CounterValue, float64(stat.Calls), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.totalTimeSeconds, prometheus.CounterValue, stat.TotalTimeSeconds, stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.minTimeSeconds, prometheus.GaugeValue, stat.MinTimeSeconds, stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.maxTimeSeconds, prometheus.GaugeValue, stat.MaxTimeSeconds, stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.meanTimeSeconds, prometheus.GaugeValue, stat.MeanTimeSeconds, stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.stdDevTimeSeconds, prometheus.GaugeValue, stat.StdDevTimeSeconds, stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.rows, prometheus.CounterValue, float64(stat.Rows), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.sharedBlksHit, prometheus.CounterValue, float64(stat.SharedBlksHit), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.sharedBlksRead, prometheus.CounterValue, float64(stat.SharedBlksRead), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.sharedBlksDirtied, prometheus.CounterValue, float64(stat.SharedBlksDirtied), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.sharedBlksWritten, prometheus.CounterValue, float64(stat.SharedBlksWritten), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.localBlksHit, prometheus.CounterValue, float64(stat.LocalBlksHit), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.localBlksRead, prometheus.CounterValue, float64(stat.LocalBlksRead), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.localBlksDirtied, prometheus.CounterValue, float64(stat.LocalBlksDirtied), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.localBlksWritten, prometheus.CounterValue, float64(stat.LocalBlksWritten), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.tempBlksRead, prometheus.CounterValue, float64(stat.TempBlksRead), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.tempBlksWritten, prometheus.CounterValue, float64(stat.TempBlksWritten), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.blkReadTimeSeconds, prometheus.CounterValue, float64(stat.BlkReadTimeSeconds), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
		ch <- prometheus.MustNewConstMetric(c.blkWriteTimeSeconds, prometheus.CounterValue, float64(stat.BlkWriteTimeSeconds), stat.Database, stat.RolName, stat.DatName, strconv.Itoa(stat.QueryID), stat.Query)
	}
	return nil
}
