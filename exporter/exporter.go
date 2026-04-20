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

// namespace is the Prometheus metric namespace prefix used for this
// exporter's self-metrics (pg_stat_up, pg_stat_exporter_scrapes_total).
// Kept as a var — not a const — so Opts.MetricPrefix can flip it at
// New() time to stay consistent with the collector-level prefix.
var namespace = "pg_stat"

// Opts for the exporter.
type Opts struct {
	DBOpts []db.Opts
	// Name is used as the "database" const label on internal metrics (pg_stat_up,
	// pg_stat_exporter_scrapes_total) to disambiguate when multiple exporters are
	// registered in the same Prometheus registry. Optional — omit for single-exporter use.
	Name string
	// CollectionTimeout bounds a single Prometheus scrape cycle. Zero means
	// DefaultCollectionTimeout; pass -1 for no timeout. Propagated to every
	// collector via [collectors.Collector.Scrape] so a pathological query
	// cannot hang the scrape indefinitely.
	CollectionTimeout time.Duration
	// MetricPrefix overrides the default "pg_stat" metric namespace for
	// both exporter self-metrics and every registered collector. Set to
	// [collectors.MetricPrefixPg] to match postgres_exporter's naming so
	// community Grafana dashboards work unchanged. Empty value keeps the
	// default. Must be set before New() is called; changing it after has
	// no effect on already-constructed collectors.
	MetricPrefix collectors.MetricPrefix

	// EnabledCollectors, if non-empty, restricts the running collector
	// set to the listed names (see collectors.AvailableCollectors).
	// Overrides the default-enabled set. Typical use: opt in to
	// pg_stat_statements without pulling every other collector.
	EnabledCollectors []string
	// DisabledCollectors subtracts names from the resolved collector
	// set. Applies after EnabledCollectors — a name in both wins for
	// Disabled. Unknown names are logged as a warning, not fatal.
	DisabledCollectors []string
}

// DefaultCollectionTimeout is the per-scrape deadline if Opts.CollectionTimeout
// is zero. Matches prometheus-community/postgres_exporter's --collection-timeout
// default so operators migrating from postgres_exporter get the same behaviour.
const DefaultCollectionTimeout = 1 * time.Minute

// Exporter collects PostgreSQL metrics and exports them via prometheus.
type Exporter struct {
	dbClients  []*db.Client
	collectors []collectors.Collector

	collectionTimeout time.Duration

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
	// Apply the metric-prefix override BEFORE DefaultCollectors runs —
	// collector descriptors bake in the prefix at construction time.
	if opts.MetricPrefix != "" {
		collectors.SetMetricPrefix(opts.MetricPrefix)
		namespace = string(opts.MetricPrefix)
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

	collectionTimeout := opts.CollectionTimeout
	if collectionTimeout == 0 {
		collectionTimeout = DefaultCollectionTimeout
	}

	resolvedCollectors, resolveErr := collectors.ResolveCollectors(dbClients, opts.EnabledCollectors, opts.DisabledCollectors)
	if resolveErr != nil {
		// Non-fatal: typos shouldn't brick the exporter. Log and use
		// whatever subset resolved cleanly.
		log.Warnf("collector resolution: %v", resolveErr)
	}

	return &Exporter{
		dbClients:         dbClients,
		collectors:        resolvedCollectors,
		collectionTimeout: collectionTimeout,

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

// ExtendFromYAMLFile loads a YAML file of CollectorSpecs (see the
// collectors.LoadSpecsFromFile godoc for the format) and registers one
// SpecCollector per spec on this exporter. Returns the first parse/
// validation error encountered; no collectors are registered on error.
//
// This is the one-liner entrypoint for operators who want to extend
// pgxporter with custom SQL without writing Go — a direct replacement
// for postgres_exporter's deprecated queries.yaml pattern.
func (e *Exporter) ExtendFromYAMLFile(path string) error {
	specs, err := collectors.LoadSpecsFromFile(path)
	if err != nil {
		return err
	}
	extra := make([]collectors.Collector, 0, len(specs))
	for _, s := range specs {
		extra = append(extra, collectors.NewSpecCollector(s, e.dbClients))
	}
	e.collectors = append(e.collectors, extra...)
	return nil
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

// Collect implements the prometheus.Collector interface.
//
// It runs every registered collector in parallel under a per-scrape deadline
// sourced from [Opts.CollectionTimeout]. Collector errors are logged and
// surfaced as pg_stat_up=0; a single slow collector cannot hang the scrape.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	defer func() {
		log.Infof("exporter collect took %dms", time.Now().Sub(start).Milliseconds())
	}()
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// prometheus.Collector.Collect has no ctx argument, so we derive one here.
	// Every collector honours this deadline via [collectors.Collector.Scrape].
	ctx, cancel := e.scrapeContext()
	defer cancel()

	e.totalScrapes.Inc()
	up := 1
	group, gctx := errgroup.WithContext(ctx)
	for _, collector := range e.collectors {
		collector := collector
		group.Go(func() error { return collector.Scrape(gctx, ch) })
	}
	if err := group.Wait(); err != nil {
		up = 0
		log.Errorf("collecting: %v", err)
	}
	ch <- prometheus.MustNewConstMetric(e.up.Desc(), prometheus.GaugeValue, float64(up))
	ch <- e.totalScrapes
}

// scrapeContext returns a context bounded by [Exporter.collectionTimeout].
// A negative timeout disables the deadline (for debugging); zero would be
// caught upstream by [New] and replaced with [DefaultCollectionTimeout].
func (e *Exporter) scrapeContext() (context.Context, context.CancelFunc) {
	if e.collectionTimeout < 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), e.collectionTimeout)
}
