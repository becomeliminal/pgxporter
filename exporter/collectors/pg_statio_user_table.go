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

// PgStatIOUserTableCollector collects from pg_statio_user_tables.
type PgStatIOUserTableCollector struct {
	dbClients []*db.Client

	heapBlksRead  *counterDelta
	heapBlksHit   *counterDelta
	idxBlksRead   *counterDelta
	idxBlksHit    *counterDelta
	toastBlksRead *counterDelta
	toastBlksHit  *counterDelta
	tidxBlksRead  *counterDelta
	tidxBlksHit   *counterDelta
}

// NewPgStatIOUserTableCollector instantiates and returns a new PgStatIOUserTableCollector.
func NewPgStatIOUserTableCollector(dbClients []*db.Client) *PgStatIOUserTableCollector {
	variableLabels := []string{"database", "schemaname", "relname"}
	counter := counterFactoryIO(userTablesSubSystem, variableLabels)
	return &PgStatIOUserTableCollector{
		dbClients: dbClients,

		heapBlksRead:  counter("heap_blks_read", "Number of disk blocks read from this table"),
		heapBlksHit:   counter("heap_blks_hit", "Number of buffer hits in this table"),
		idxBlksRead:   counter("idx_blks_read", "Number of disk blocks read from all indexes on this table"),
		idxBlksHit:    counter("idx_blks_hit", "Number of buffer hits in all indexes on this table"),
		toastBlksRead: counter("toast_blks_read", "Number of disk blocks read from this table's TOAST table (if any)"),
		toastBlksHit:  counter("toast_blks_hit", "Number of buffer hits in this table's TOAST table (if any)"),
		tidxBlksRead:  counter("tidx_blks_read", "Number of disk blocks read from this table's TOAST table indexes (if any)"),
		tidxBlksHit:   counter("tidx_blks_hit", "Number of buffer hits in this table's TOAST table indexes (if any)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatIOUserTableCollector) Describe(ch chan<- *prometheus.Desc) {
	c.heapBlksRead.Describe(ch)
	c.heapBlksHit.Describe(ch)
	c.idxBlksRead.Describe(ch)
	c.idxBlksHit.Describe(ch)
	c.toastBlksRead.Describe(ch)
	c.toastBlksHit.Describe(ch)
	c.tidxBlksRead.Describe(ch)
	c.tidxBlksHit.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatIOUserTableCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatIOUserTableCollector) collectInto(ch chan<- prometheus.Metric) {
	c.heapBlksRead.Collect(ch)
	c.heapBlksHit.Collect(ch)
	c.idxBlksRead.Collect(ch)
	c.idxBlksHit.Collect(ch)
	c.toastBlksRead.Collect(ch)
	c.toastBlksHit.Collect(ch)
	c.tidxBlksRead.Collect(ch)
	c.tidxBlksHit.Collect(ch)
}

func (c *PgStatIOUserTableCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	userTableStats, err := dbClient.SelectPgStatIOUserTables(ctx)
	if err != nil {
		return fmt.Errorf("user table stats: %w", err)
	}
	c.emit(userTableStats)
	return nil
}

// emit turns scanned pg_statio_user_tables rows into metrics, skipping NULL
// counter columns. Separated from scrape for unit-test coverage.
func (c *PgStatIOUserTableCollector) emit(stats []*model.PgStatIOUserTable) {
	for _, stat := range stats {
		labels := []string{stat.Database.String, stat.SchemaName.String, stat.RelName.String}
		emit := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), labels...)
			}
		}
		emit(c.heapBlksRead, stat.HeapBlksRead)
		emit(c.heapBlksHit, stat.HeapBlksHit)
		emit(c.idxBlksRead, stat.IndexBlksRead)
		emit(c.idxBlksHit, stat.IndexBlksHit)
		emit(c.toastBlksRead, stat.ToastBlksRead)
		emit(c.toastBlksHit, stat.ToastBlksHit)
		emit(c.tidxBlksRead, stat.TidxBlksRead)
		emit(c.tidxBlksHit, stat.TidxBlksHit)
	}
}
