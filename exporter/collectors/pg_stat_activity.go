package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatActivityCollector emits aggregate metrics from pg_stat_activity.
// Labels: database, datname, state, wait_event_type, backend_type. Per-
// backend detail (pid, query text, usename, client_addr) is deliberately
// not exposed — it's unbounded and better suited to logs than metrics.
type PgStatActivityCollector struct {
	dbClients []*db.Client

	count                   *prometheus.Desc
	maxTxDurationSeconds    *prometheus.Desc
	maxQueryDurationSeconds *prometheus.Desc
	maxBackendAgeSeconds    *prometheus.Desc
}

// NewPgStatActivityCollector instantiates and returns a new PgStatActivityCollector.
func NewPgStatActivityCollector(dbClients []*db.Client) *PgStatActivityCollector {
	labels := []string{"database", "datname", "state", "wait_event_type", "backend_type"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, activitySubSystem, name), help, labels, nil)
	}
	return &PgStatActivityCollector{
		dbClients: dbClients,

		count:                   desc("count", "Connections in this (state, wait_event_type, backend_type) bucket"),
		maxTxDurationSeconds:    desc("max_tx_duration_seconds", "Longest open transaction in this bucket"),
		maxQueryDurationSeconds: desc("max_query_duration_seconds", "Longest running query in this bucket"),
		maxBackendAgeSeconds:    desc("max_backend_age_seconds", "Oldest backend_start in this bucket"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatActivityCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.count
	ch <- c.maxTxDurationSeconds
	ch <- c.maxQueryDurationSeconds
	ch <- c.maxBackendAgeSeconds
}

// Scrape implements our Scraper interface.
func (c *PgStatActivityCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("activity scrape took %dms", time.Since(start).Milliseconds())
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

func (c *PgStatActivityCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatActivity(ctx)
	if err != nil {
		return fmt.Errorf("activity stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatActivityCollector) emit(stats []*model.PgStatActivity, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.DatName.String,
			stat.State.String,
			stat.WaitEventType.String,
			stat.BackendType.String,
		}
		emitInt := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Int64), labels...)
			}
		}
		emitFloat := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v.Float64, labels...)
			}
		}
		emitInt(c.count, stat.Count)
		emitFloat(c.maxTxDurationSeconds, stat.MaxTxDurationSeconds)
		emitFloat(c.maxQueryDurationSeconds, stat.MaxQueryDurationSeconds)
		emitFloat(c.maxBackendAgeSeconds, stat.MaxBackendAgeSeconds)
	}
}
