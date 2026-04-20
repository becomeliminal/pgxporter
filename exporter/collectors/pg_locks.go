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

// PgLocksCollector emits aggregate lock counts plus a blocking-chain
// summary. The blocking-chain query uses pg_blocking_pids() which is
// O(N²) in backend count — on a cluster with heavy connection churn
// the summary alone may dominate the scrape budget; collector stays on
// by default because the signal is usually indispensable and the cost
// is bounded by the scrape-level context deadline.
type PgLocksCollector struct {
	dbClients []*db.Client

	count           *prometheus.GaugeVec
	blockedBackends *prometheus.GaugeVec
	blockerEdges    *prometheus.GaugeVec
}

// NewPgLocksCollector instantiates and returns a new PgLocksCollector.
func NewPgLocksCollector(dbClients []*db.Client) *PgLocksCollector {
	lockLabels := []string{"database", "datname", "mode", "locktype", "granted"}
	summaryLabels := []string{"database"}
	lockGauge := gaugeFactory(locksSubSystem, lockLabels)
	summaryGauge := gaugeFactory(locksSubSystem, summaryLabels)
	return &PgLocksCollector{
		dbClients:       dbClients,
		count:           lockGauge("count", "Number of locks in this (mode, locktype, granted) bucket"),
		blockedBackends: summaryGauge("blocked_backends", "Number of backends currently waiting on another backend"),
		blockerEdges:    summaryGauge("blocker_edges", "Total (blocked, blocker) pairs across the cluster — sum of |pg_blocking_pids(pid)|"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgLocksCollector) Describe(ch chan<- *prometheus.Desc) {
	c.count.Describe(ch)
	c.blockedBackends.Describe(ch)
	c.blockerEdges.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgLocksCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgLocksCollector) collectInto(ch chan<- prometheus.Metric) {
	c.count.Collect(ch)
	c.blockedBackends.Collect(ch)
	c.blockerEdges.Collect(ch)
}

func (c *PgLocksCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	locks, err := dbClient.SelectPgLocks(ctx)
	if err != nil {
		return fmt.Errorf("lock stats: %w", err)
	}
	summary, err := dbClient.SelectPgLocksBlockingSummary(ctx)
	if err != nil {
		return fmt.Errorf("lock blocking summary: %w", err)
	}
	c.emit(locks, summary)
	return nil
}

func (c *PgLocksCollector) emit(locks []*model.PgLock, summary []*model.PgLocksBlockingSummary) {
	for _, stat := range locks {
		if !stat.Count.Valid {
			continue
		}
		granted := ""
		if stat.Granted.Valid {
			granted = strconv.FormatBool(stat.Granted.Bool)
		}
		c.count.WithLabelValues(
			stat.Database.String, stat.DatName.String, stat.Mode.String,
			stat.LockType.String, granted,
		).Set(float64(stat.Count.Int64))
	}
	for _, s := range summary {
		emitInt := func(vec *prometheus.GaugeVec, v pgtype.Int8) {
			if v.Valid {
				vec.WithLabelValues(s.Database.String).Set(float64(v.Int64))
			}
		}
		emitInt(c.blockedBackends, s.BlockedBackends)
		emitInt(c.blockerEdges, s.BlockerEdges)
	}
}
