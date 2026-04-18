package collectors

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/jackc/pgx/v5/pgtype"
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
		queryID := ""
		if stat.QueryID.Valid {
			queryID = strconv.FormatInt(stat.QueryID.Int64, 10)
		}
		labels := []string{stat.Database.String, stat.RolName.String, stat.DatName.String, queryID, stat.Query.String}
		emitInt := func(desc *prometheus.Desc, valueType prometheus.ValueType, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, valueType, float64(v.Int64), labels...)
			}
		}
		emitFloat := func(desc *prometheus.Desc, valueType prometheus.ValueType, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, valueType, v.Float64, labels...)
			}
		}
		emitInt(c.calls, prometheus.CounterValue, stat.Calls)
		emitFloat(c.totalTimeSeconds, prometheus.CounterValue, stat.TotalTimeSeconds)
		emitFloat(c.minTimeSeconds, prometheus.GaugeValue, stat.MinTimeSeconds)
		emitFloat(c.maxTimeSeconds, prometheus.GaugeValue, stat.MaxTimeSeconds)
		emitFloat(c.meanTimeSeconds, prometheus.GaugeValue, stat.MeanTimeSeconds)
		emitFloat(c.stdDevTimeSeconds, prometheus.GaugeValue, stat.StdDevTimeSeconds)
		emitInt(c.rows, prometheus.CounterValue, stat.Rows)
		emitInt(c.sharedBlksHit, prometheus.CounterValue, stat.SharedBlksHit)
		emitInt(c.sharedBlksRead, prometheus.CounterValue, stat.SharedBlksRead)
		emitInt(c.sharedBlksDirtied, prometheus.CounterValue, stat.SharedBlksDirtied)
		emitInt(c.sharedBlksWritten, prometheus.CounterValue, stat.SharedBlksWritten)
		emitInt(c.localBlksHit, prometheus.CounterValue, stat.LocalBlksHit)
		emitInt(c.localBlksRead, prometheus.CounterValue, stat.LocalBlksRead)
		emitInt(c.localBlksDirtied, prometheus.CounterValue, stat.LocalBlksDirtied)
		emitInt(c.localBlksWritten, prometheus.CounterValue, stat.LocalBlksWritten)
		emitInt(c.tempBlksRead, prometheus.CounterValue, stat.TempBlksRead)
		emitInt(c.tempBlksWritten, prometheus.CounterValue, stat.TempBlksWritten)
		emitFloat(c.blkReadTimeSeconds, prometheus.CounterValue, stat.BlkReadTimeSeconds)
		emitFloat(c.blkWriteTimeSeconds, prometheus.CounterValue, stat.BlkWriteTimeSeconds)
	}
	return nil
}
