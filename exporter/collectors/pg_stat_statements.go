package collectors

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatStatementsCollector collects from pg_stat_statements.
type PgStatStatementsCollector struct {
	dbClients []*db.Client

	calls               *counterDelta
	totalTimeSeconds    *counterDelta
	minTimeSeconds      *prometheus.GaugeVec
	maxTimeSeconds      *prometheus.GaugeVec
	meanTimeSeconds     *prometheus.GaugeVec
	stdDevTimeSeconds   *prometheus.GaugeVec
	rows                *counterDelta
	sharedBlksHit       *counterDelta
	sharedBlksRead      *counterDelta
	sharedBlksDirtied   *counterDelta
	sharedBlksWritten   *counterDelta
	localBlksHit        *counterDelta
	localBlksRead       *counterDelta
	localBlksDirtied    *counterDelta
	localBlksWritten    *counterDelta
	tempBlksRead        *counterDelta
	tempBlksWritten     *counterDelta
	blkReadTimeSeconds  *counterDelta
	blkWriteTimeSeconds *counterDelta
}

// NewPgStatStatementsCollector instantiates and returns a new PgStatStatementsCollector.
func NewPgStatStatementsCollector(dbClients []*db.Client) *PgStatStatementsCollector {
	variableLabels := []string{"database", "rolname", "datname", "queryid", "query"}
	counter := counterFactory(statementsSubSystem, variableLabels)
	gauge := gaugeFactory(statementsSubSystem, variableLabels)
	return &PgStatStatementsCollector{
		dbClients: dbClients,

		calls:               counter("calls", ""),
		totalTimeSeconds:    counter("total_time_seconds", ""),
		minTimeSeconds:      gauge("min_time_seconds", ""),
		maxTimeSeconds:      gauge("max_time_seconds", ""),
		meanTimeSeconds:     gauge("mean_time_seconds", ""),
		stdDevTimeSeconds:   gauge("std_dev_time_seconds", ""),
		rows:                counter("rows", ""),
		sharedBlksHit:       counter("shared_blks_hit", ""),
		sharedBlksRead:      counter("shared_blks_read", ""),
		sharedBlksDirtied:   counter("shared_blks_dirtied", ""),
		sharedBlksWritten:   counter("shared_blks_written", ""),
		localBlksHit:        counter("local_blks_hit", ""),
		localBlksRead:       counter("local_blks_read", ""),
		localBlksDirtied:    counter("local_blks_dirtied", ""),
		localBlksWritten:    counter("local_blks_written", ""),
		tempBlksRead:        counter("templ_blks_read", ""),
		tempBlksWritten:     counter("temp_blks_written", ""),
		blkReadTimeSeconds:  counter("blk_read_time_seconds", ""),
		blkWriteTimeSeconds: counter("blk_write_time_seconds", ""),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatStatementsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.calls.Describe(ch)
	c.totalTimeSeconds.Describe(ch)
	c.minTimeSeconds.Describe(ch)
	c.maxTimeSeconds.Describe(ch)
	c.meanTimeSeconds.Describe(ch)
	c.stdDevTimeSeconds.Describe(ch)
	c.rows.Describe(ch)
	c.sharedBlksHit.Describe(ch)
	c.sharedBlksRead.Describe(ch)
	c.sharedBlksDirtied.Describe(ch)
	c.sharedBlksWritten.Describe(ch)
	c.localBlksHit.Describe(ch)
	c.localBlksRead.Describe(ch)
	c.localBlksDirtied.Describe(ch)
	c.localBlksWritten.Describe(ch)
	c.tempBlksRead.Describe(ch)
	c.tempBlksWritten.Describe(ch)
	c.blkReadTimeSeconds.Describe(ch)
	c.blkWriteTimeSeconds.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatStatementsCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatStatementsCollector) collectInto(ch chan<- prometheus.Metric) {
	c.calls.Collect(ch)
	c.totalTimeSeconds.Collect(ch)
	c.minTimeSeconds.Collect(ch)
	c.maxTimeSeconds.Collect(ch)
	c.meanTimeSeconds.Collect(ch)
	c.stdDevTimeSeconds.Collect(ch)
	c.rows.Collect(ch)
	c.sharedBlksHit.Collect(ch)
	c.sharedBlksRead.Collect(ch)
	c.sharedBlksDirtied.Collect(ch)
	c.sharedBlksWritten.Collect(ch)
	c.localBlksHit.Collect(ch)
	c.localBlksRead.Collect(ch)
	c.localBlksDirtied.Collect(ch)
	c.localBlksWritten.Collect(ch)
	c.tempBlksRead.Collect(ch)
	c.tempBlksWritten.Collect(ch)
	c.blkReadTimeSeconds.Collect(ch)
	c.blkWriteTimeSeconds.Collect(ch)
}

func (c *PgStatStatementsCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	statementStats, err := dbClient.SelectPgStatStatements(ctx)
	if err != nil {
		return fmt.Errorf("statement stats: %w", err)
	}
	c.emit(statementStats)
	return nil
}

// emit turns scanned pg_stat_statements rows into metrics, skipping NULL
// counter / timing columns. Separated from scrape for unit-test coverage.
func (c *PgStatStatementsCollector) emit(stats []*model.PgStatStatement) {
	for _, stat := range stats {
		queryID := ""
		if stat.QueryID.Valid {
			queryID = strconv.FormatInt(stat.QueryID.Int64, 10)
		}
		labels := []string{stat.Database.String, stat.RolName.String, stat.DatName.String, queryID, stat.Query.String}
		emitCounterInt := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), labels...)
			}
		}
		emitCounterFloat := func(cd *counterDelta, v pgtype.Float8) {
			if v.Valid {
				cd.Observe(v.Float64, labels...)
			}
		}
		emitGaugeFloat := func(vec *prometheus.GaugeVec, v pgtype.Float8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(v.Float64)
			}
		}
		emitCounterInt(c.calls, stat.Calls)
		emitCounterFloat(c.totalTimeSeconds, stat.TotalTimeSeconds)
		emitGaugeFloat(c.minTimeSeconds, stat.MinTimeSeconds)
		emitGaugeFloat(c.maxTimeSeconds, stat.MaxTimeSeconds)
		emitGaugeFloat(c.meanTimeSeconds, stat.MeanTimeSeconds)
		emitGaugeFloat(c.stdDevTimeSeconds, stat.StdDevTimeSeconds)
		emitCounterInt(c.rows, stat.Rows)
		emitCounterInt(c.sharedBlksHit, stat.SharedBlksHit)
		emitCounterInt(c.sharedBlksRead, stat.SharedBlksRead)
		emitCounterInt(c.sharedBlksDirtied, stat.SharedBlksDirtied)
		emitCounterInt(c.sharedBlksWritten, stat.SharedBlksWritten)
		emitCounterInt(c.localBlksHit, stat.LocalBlksHit)
		emitCounterInt(c.localBlksRead, stat.LocalBlksRead)
		emitCounterInt(c.localBlksDirtied, stat.LocalBlksDirtied)
		emitCounterInt(c.localBlksWritten, stat.LocalBlksWritten)
		emitCounterInt(c.tempBlksRead, stat.TempBlksRead)
		emitCounterInt(c.tempBlksWritten, stat.TempBlksWritten)
		emitCounterFloat(c.blkReadTimeSeconds, stat.BlkReadTimeSeconds)
		emitCounterFloat(c.blkWriteTimeSeconds, stat.BlkWriteTimeSeconds)
	}
}
