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

// PgStatIOUserTableCollector collects from pg_statio_user_tables.
type PgStatIOUserTableCollector struct {
	dbClients []*db.Client
	mutex     sync.RWMutex

	heapBlksRead  *prometheus.Desc
	heapBlksHit   *prometheus.Desc
	idxBlksRead   *prometheus.Desc
	idxBlksHit    *prometheus.Desc
	toastBlksRead *prometheus.Desc
	toastBlksHit  *prometheus.Desc
	tidxBlksRead  *prometheus.Desc
	tidxBlksHit   *prometheus.Desc
}

// NewPgStatIOUserTableCollector instantiates and returns a new PgStatIOUserTableCollector.
func NewPgStatIOUserTableCollector(dbClients []*db.Client) *PgStatIOUserTableCollector {
	variableLabels := []string{"database", "schemaname", "relname"}
	return &PgStatIOUserTableCollector{
		dbClients: dbClients,

		heapBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "heap_blks_read"),
			"Number of disk blocks read from this table",
			variableLabels,
			nil,
		),
		heapBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "heap_blks_hit"),
			"Number of buffer hits in this table",
			variableLabels,
			nil,
		),
		idxBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "idx_blks_read"),
			"Number of disk blocks read from all indexes on this table",
			variableLabels,
			nil,
		),
		idxBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "idx_blks_hit"),
			"Number of buffer hits in all indexes on this table",
			variableLabels,
			nil,
		),

		toastBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "toast_blks_read"),
			"Number of disk blocks read from this table's TOAST table (if any)",
			variableLabels,
			nil,
		),
		toastBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "toast_blks_hit"),
			"Number of buffer hits in this table's TOAST table (if any)",
			variableLabels,
			nil,
		),
		tidxBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "tidx_blks_read"),
			"Number of disk blocks read from this table's TOAST table indexes (if any)",
			variableLabels,
			nil,
		),
		tidxBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespaceIO, userTablesSubSystem, "tidx_blks_hit"),
			"Number of buffer hits in this table's TOAST table indexes (if any)",
			variableLabels,
			nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatIOUserTableCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.heapBlksRead
	ch <- c.heapBlksHit
	ch <- c.idxBlksRead
	ch <- c.idxBlksHit
	ch <- c.toastBlksRead
	ch <- c.toastBlksHit
	ch <- c.tidxBlksRead
	ch <- c.tidxBlksHit
}

// Collect implements the promtheus.Collector.
func (c *PgStatIOUserTableCollector) Collect(ch chan<- prometheus.Metric) {
	_ = c.Scrape(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatIOUserTableCollector) Scrape(ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("I/O user table scrape took %dms", time.Now().Sub(start).Milliseconds())
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

func (c *PgStatIOUserTableCollector) scrape(dbClient *db.Client, ch chan<- prometheus.Metric) error {
	userTableStats, err := dbClient.SelectPgStatIOUserTables(context.Background())
	if err != nil {
		return fmt.Errorf("user table stats: %w", err)
	}
	for _, stat := range userTableStats {
		labels := []string{stat.Database.String, stat.SchemaName.String, stat.RelName.String}
		emit := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(v.Int64), labels...)
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
	return nil
}
