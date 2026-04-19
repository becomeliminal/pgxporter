package exporter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/collectors"
	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/logging"
)

var log = logging.NewLogger()

const namespace = "pg_stat"

// Opts for the exporter.
type Opts struct {
	DBOpts []db.Opts
	// Name is used as the "database" const label on internal metrics (pg_stat_up,
	// pg_stat_exporter_scrapes_total) to disambiguate when multiple exporters are
	// registered in the same Prometheus registry. Optional — omit for single-exporter use.
	Name string
}

// Exporter collects PostgreSQL metrics and exports them via prometheus.
type Exporter struct {
	dbClients  []*db.Client
	collectors []collectors.Collector

	// Internal metrics.
	up           prometheus.Gauge
	totalScrapes prometheus.Counter

	mutex sync.RWMutex
}

// MustNew instantiates and returns a new Exporter or panics.
func MustNew(ctx context.Context, opts Opts) *Exporter {
	exporter, err := New(ctx, opts)
	if err != nil {
		panic(err)
	}
	return exporter
}

// New instaniates and returns a new Exporter.
func New(ctx context.Context, opts Opts) (*Exporter, error) {
	if len(opts.DBOpts) < 1 {
		return nil, fmt.Errorf("missing db opts")
	}
	dbClients := make([]*db.Client, 0, len(opts.DBOpts))
	for _, dbOpt := range opts.DBOpts {
		dbClient, err := db.New(ctx, dbOpt)
		if err != nil {
			return nil, fmt.Errorf("creating exporter: %w", err)
		}
		dbClients = append(dbClients, dbClient)
	}
	var constLabels prometheus.Labels
	if opts.Name != "" {
		constLabels = prometheus.Labels{"database": opts.Name}
	}

	return &Exporter{
		dbClients:  dbClients,
		collectors: collectors.DefaultCollectors(dbClients),

		// Internal metrics.
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "up",
			Help:        "Was the last scrape of PostgreSQL successful.",
			ConstLabels: constLabels,
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        "exporter_scrapes_total",
			Help:        "Current total PostgreSQL scrapes",
			ConstLabels: constLabels,
		}),
	}, nil
}

// WithCustomCollectors lets the exporter scrape custom metrics.
func (e *Exporter) WithCustomCollectors(collectors ...collectors.Collector) *Exporter {
	e.collectors = append(e.collectors, collectors...)
	return e
}

// Register the exporter.
func (e *Exporter) Register() { prometheus.MustRegister(e) }

// HealthCheck pings PostgreSQL.
func (e *Exporter) HealthCheck(ctx context.Context) error {
	group := errgroup.Group{}
	for _, dbClient := range e.dbClients {
		ctx := ctx
		dbClient := dbClient
		group.Go(func() error { return dbClient.CheckConnection(ctx) })
	}
	return group.Wait()
}

// Describe implements the prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// Internal metrics.
	ch <- e.up.Desc()
	ch <- e.totalScrapes.Desc()
	for _, collector := range e.collectors {
		collector.Describe(ch)
	}
}

// Collect implements the promtheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	defer func() {
		log.Infof("exporter collect took %dms", time.Now().Sub(start).Milliseconds())
	}()
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.totalScrapes.Inc()
	up := 1
	group := errgroup.Group{}
	for _, collector := range e.collectors {
		collector := collector
		group.Go(func() error { return collector.Scrape(ch) })
	}
	if err := group.Wait(); err != nil {
		up = 0
		log.Errorf("collecting: %v", err)
	}
	ch <- prometheus.MustNewConstMetric(e.up.Desc(), prometheus.GaugeValue, float64(up))
	ch <- e.totalScrapes
}
