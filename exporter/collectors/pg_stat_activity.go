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

// PgStatActivityCollector emits aggregate metrics from pg_stat_activity.
// Labels: database, datname, state, wait_event_type, backend_type. Per-
// backend detail (pid, query text, usename, client_addr) is deliberately
// not exposed — it's unbounded and better suited to logs than metrics.
type PgStatActivityCollector struct {
	dbClients []*db.Client

	count                   *prometheus.GaugeVec
	maxTxDurationSeconds    *prometheus.GaugeVec
	maxQueryDurationSeconds *prometheus.GaugeVec
	maxBackendAgeSeconds    *prometheus.GaugeVec
}

// NewPgStatActivityCollector instantiates and returns a new PgStatActivityCollector.
func NewPgStatActivityCollector(dbClients []*db.Client) *PgStatActivityCollector {
	labels := []string{"database", "datname", "state", "wait_event_type", "backend_type"}
	gauge := gaugeFactory(activitySubSystem, labels)
	return &PgStatActivityCollector{
		dbClients: dbClients,

		count:                   gauge("count", "Connections in this (state, wait_event_type, backend_type) bucket"),
		maxTxDurationSeconds:    gauge("max_tx_duration_seconds", "Longest open transaction in this bucket"),
		maxQueryDurationSeconds: gauge("max_query_duration_seconds", "Longest running query in this bucket"),
		maxBackendAgeSeconds:    gauge("max_backend_age_seconds", "Oldest backend_start in this bucket"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatActivityCollector) Describe(ch chan<- *prometheus.Desc) {
	c.count.Describe(ch)
	c.maxTxDurationSeconds.Describe(ch)
	c.maxQueryDurationSeconds.Describe(ch)
	c.maxBackendAgeSeconds.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatActivityCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgStatActivityCollector) collectInto(ch chan<- prometheus.Metric) {
	c.count.Collect(ch)
	c.maxTxDurationSeconds.Collect(ch)
	c.maxQueryDurationSeconds.Collect(ch)
	c.maxBackendAgeSeconds.Collect(ch)
}

func (c *PgStatActivityCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatActivity(ctx)
	if err != nil {
		return fmt.Errorf("activity stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatActivityCollector) emit(stats []*model.PgStatActivity) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.DatName.String,
			stat.State.String,
			stat.WaitEventType.String,
			stat.BackendType.String,
		}
		emitInt := func(vec *prometheus.GaugeVec, v pgtype.Int8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(float64(v.Int64))
			}
		}
		emitFloat := func(vec *prometheus.GaugeVec, v pgtype.Float8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(v.Float64)
			}
		}
		emitInt(c.count, stat.Count)
		emitFloat(c.maxTxDurationSeconds, stat.MaxTxDurationSeconds)
		emitFloat(c.maxQueryDurationSeconds, stat.MaxQueryDurationSeconds)
		emitFloat(c.maxBackendAgeSeconds, stat.MaxBackendAgeSeconds)
	}
}
