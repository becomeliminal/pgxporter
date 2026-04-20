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
// exporter's self-metrics (pg_stat_up, pg_stat_exporter_scrapes_total,
// and the per-collector self-instrumentation added in M6). Kept as a
// var — not a const — so Opts.MetricPrefix can flip it at New() time
// to stay consistent with the collector-level prefix.
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
	collectors []collectors.NamedCollector

	collectionTimeout time.Duration

	// Self-metrics, emitted by every Collect call.
	up                prometheus.Gauge
	totalScrapes      prometheus.Counter
	scrapeDuration    *prometheus.HistogramVec
	scrapeErrors      *prometheus.CounterVec
	metricCardinality *prometheus.GaugeVec

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
		log.Warn("collector resolution", "err", resolveErr)
	}

	e := &Exporter{
		dbClients:         dbClients,
		collectors:        resolvedCollectors,
		collectionTimeout: collectionTimeout,

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
		// Scrape-duration buckets cover the realistic range for a
		// single-collector scrape: fast (local PG) is ~1 ms, busy
		// (pg_locks with blocking-chain on a loaded cluster) can hit
		// ~1 s. 10 s matches the default collection timeout.
		scrapeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   namespace,
			Name:        "scrape_duration_seconds",
			Help:        "Time spent scraping a collector, seconds.",
			Buckets:     []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			ConstLabels: constLabels,
		}, []string{"collector"}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        "scrape_errors_total",
			Help:        "Scrape errors by collector.",
			ConstLabels: constLabels,
		}, []string{"collector"}),
		// metricCardinality is the count of metrics emitted by a
		// collector on its most recent scrape. An early alert signal
		// for cardinality explosions before Prometheus itself chokes.
		metricCardinality: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "metric_cardinality",
			Help:        "Metric count emitted by a collector on its last scrape.",
			ConstLabels: constLabels,
		}, []string{"collector"}),
	}

	// Pre-initialise the error counter at zero for every resolved
	// collector so rate(pg_stat_scrape_errors_total[5m]) gives a clean
	// 0-rate signal on healthy exporters instead of "no data". The
	// Counter's label-set only materialises after the first touch, so
	// we explicitly .WithLabelValues() here to create the series.
	for _, nc := range resolvedCollectors {
		e.scrapeErrors.WithLabelValues(nc.Name)
	}
	return e, nil
}

// WithCustomCollectors lets the exporter scrape custom metrics. Custom
// collectors get a synthetic name ("custom_<N>") for self-metric labels.
func (e *Exporter) WithCustomCollectors(cs ...collectors.Collector) *Exporter {
	for i, c := range cs {
		e.collectors = append(e.collectors, collectors.NamedCollector{
			Name:      fmt.Sprintf("custom_%d", len(e.collectors)+i),
			Collector: c,
		})
	}
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
	for _, s := range specs {
		e.collectors = append(e.collectors, collectors.NamedCollector{
			Name:      "spec_" + s.Subsystem,
			Collector: collectors.NewSpecCollector(s, e.dbClients),
		})
	}
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
	ch <- e.up.Desc()
	ch <- e.totalScrapes.Desc()
	e.scrapeDuration.Describe(ch)
	e.scrapeErrors.Describe(ch)
	e.metricCardinality.Describe(ch)
	for _, nc := range e.collectors {
		nc.Collector.Describe(ch)
	}
}

// Collect implements the prometheus.Collector interface.
//
// It runs every registered collector in parallel under a per-scrape deadline
// sourced from [Opts.CollectionTimeout]. Collector errors are logged and
// surfaced as pg_stat_up=0; a single slow collector cannot hang the scrape.
//
// Each collector is instrumented with three self-metrics emitted at the
// end of the scrape: pg_stat_scrape_duration_seconds (histogram),
// pg_stat_scrape_errors_total (counter), pg_stat_metric_cardinality
// (gauge — count of metrics emitted on this scrape).
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	ctx, cancel := e.scrapeContext()
	defer cancel()

	e.totalScrapes.Inc()
	up := 1
	group, gctx := errgroup.WithContext(ctx)
	for _, nc := range e.collectors {
		nc := nc
		group.Go(func() error {
			return e.instrument(gctx, nc, ch)
		})
	}
	if err := group.Wait(); err != nil {
		up = 0
		log.Error("collecting", "err", err)
	}
	ch <- prometheus.MustNewConstMetric(e.up.Desc(), prometheus.GaugeValue, float64(up))
	ch <- e.totalScrapes
	e.scrapeDuration.Collect(ch)
	e.scrapeErrors.Collect(ch)
	e.metricCardinality.Collect(ch)
}

// instrument runs one collector's Scrape through a counting channel
// so the exporter can record per-collector duration, errors, and the
// metric-emission count. The counting adapter runs every metric through
// a small goroutine so the collector's blocking writes don't observe
// the extra hop — latency overhead is a single channel send, not a
// goroutine spin-up per metric.
func (e *Exporter) instrument(ctx context.Context, nc collectors.NamedCollector, ch chan<- prometheus.Metric) error {
	start := time.Now()
	counted := newCountingChan(ch)
	err := nc.Collector.Scrape(ctx, counted.in)
	n := counted.close()

	e.scrapeDuration.WithLabelValues(nc.Name).Observe(time.Since(start).Seconds())
	e.metricCardinality.WithLabelValues(nc.Name).Set(float64(n))
	if err != nil {
		e.scrapeErrors.WithLabelValues(nc.Name).Inc()
		return err
	}
	return nil
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

// countingChan forwards metrics to the outer channel while incrementing
// a counter. A single fan-in goroutine owns the count so writers see
// their normal channel-send semantics.
type countingChan struct {
	in   chan prometheus.Metric
	done chan int
}

func newCountingChan(dst chan<- prometheus.Metric) *countingChan {
	c := &countingChan{
		in:   make(chan prometheus.Metric, 64),
		done: make(chan int, 1),
	}
	go func() {
		n := 0
		for m := range c.in {
			dst <- m
			n++
		}
		c.done <- n
	}()
	return c
}

// close signals the forwarder to drain and returns the emitted count.
func (c *countingChan) close() int {
	close(c.in)
	return <-c.done
}
