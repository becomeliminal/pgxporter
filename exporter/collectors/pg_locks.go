package collectors

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgLocksCollector emits aggregate lock counts plus a blocking-chain
// summary. The blocking-chain query uses pg_blocking_pids() which is
// O(N²) in backend count — on a cluster with heavy connection churn
// the summary alone may dominate the scrape budget; collector stays on
// by default because the signal is usually indispensable and the cost
// is bounded by the scrape-level context deadline.
type PgLocksCollector struct {
	dbClients []*db.Client

	count           *prometheus.Desc
	blockedBackends *prometheus.Desc
	blockerEdges    *prometheus.Desc
}

// NewPgLocksCollector instantiates and returns a new PgLocksCollector.
func NewPgLocksCollector(dbClients []*db.Client) *PgLocksCollector {
	lockLabels := []string{"database", "datname", "mode", "locktype", "granted"}
	summaryLabels := []string{"database"}
	return &PgLocksCollector{
		dbClients: dbClients,

		count: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, locksSubSystem, "count"),
			"Number of locks in this (mode, locktype, granted) bucket",
			lockLabels, nil,
		),
		blockedBackends: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, locksSubSystem, "blocked_backends"),
			"Number of backends currently waiting on another backend",
			summaryLabels, nil,
		),
		blockerEdges: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, locksSubSystem, "blocker_edges"),
			"Total (blocked, blocker) pairs across the cluster — sum of |pg_blocking_pids(pid)|",
			summaryLabels, nil,
		),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgLocksCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.count
	ch <- c.blockedBackends
	ch <- c.blockerEdges
}

// Scrape implements our Scraper interface.
func (c *PgLocksCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("locks scrape took %dms", time.Since(start).Milliseconds())
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

func (c *PgLocksCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	locks, err := dbClient.SelectPgLocks(ctx)
	if err != nil {
		return fmt.Errorf("lock stats: %w", err)
	}
	summary, err := dbClient.SelectPgLocksBlockingSummary(ctx)
	if err != nil {
		return fmt.Errorf("lock blocking summary: %w", err)
	}
	c.emit(locks, summary, ch)
	return nil
}

func (c *PgLocksCollector) emit(locks []*model.PgLock, summary []*model.PgLocksBlockingSummary, ch chan<- prometheus.Metric) {
	for _, stat := range locks {
		if !stat.Count.Valid {
			continue
		}
		granted := ""
		if stat.Granted.Valid {
			granted = strconv.FormatBool(stat.Granted.Bool)
		}
		ch <- prometheus.MustNewConstMetric(c.count, prometheus.GaugeValue,
			float64(stat.Count.Int64),
			stat.Database.String, stat.DatName.String, stat.Mode.String,
			stat.LockType.String, granted,
		)
	}
	for _, s := range summary {
		emitInt := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Int64), s.Database.String)
			}
		}
		emitInt(c.blockedBackends, s.BlockedBackends)
		emitInt(c.blockerEdges, s.BlockerEdges)
	}
}
