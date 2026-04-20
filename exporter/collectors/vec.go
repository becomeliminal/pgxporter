package collectors

// Vec helpers for the emission pattern collectors use.
//
// Why: the previous pattern — `prometheus.MustNewConstMetric(desc, ...)` —
// allocates a fresh `*constMetric` and a fresh `[]*dto.LabelPair` slice
// on every emit, every scrape. Profiling showed 56% of per-scrape
// allocations came from `prometheus.MakeLabelPairs` inside that path.
// Migrating to `*prometheus.CounterVec` / `*prometheus.GaugeVec` gets
// us amortized allocation — the child metric objects (with their
// cached label pairs) persist across scrapes.
//
// But there's a semantic wrinkle: PG exposes cumulative counters as
// absolute values (pg_stat_database.xact_commit is total commits
// since last stats_reset), while `prometheus.Counter`'s public API
// only has Inc / Add — no Set. So for counters we can't just
// `.WithLabelValues(...).Set(absoluteValue)`. [counterDelta] solves
// this by tracking the last-observed absolute value per unique label
// combo and calling Add(delta) on each scrape. It handles counter
// resets (current < last, e.g. after pg_stat_reset() or a server
// restart) by deleting the Vec child and starting fresh from the
// current value — which the Prometheus server parses as a reset and
// handles correctly in rate() / increase().

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// counterDelta wraps a *prometheus.CounterVec with delta-tracking so
// callers can push absolute cumulative values (as PG exposes) while
// the underlying CounterVec sees only the delta-Add semantics it
// supports. Handles counter resets via DeleteLabelValues + fresh Add.
//
// Concurrency: safe for concurrent Observe calls. In practice
// collectors key counter metrics by database (and possibly other
// per-row labels), so two goroutines never Observe the same label
// combo simultaneously — the mutex is defensive, its contention
// negligible.
type counterDelta struct {
	vec *prometheus.CounterVec

	mu   sync.Mutex
	last map[string]float64 // key = strings.Join(labelValues, "\x00")
}

// newCounterDelta wraps a *prometheus.CounterVec.
func newCounterDelta(vec *prometheus.CounterVec) *counterDelta {
	return &counterDelta{
		vec:  vec,
		last: make(map[string]float64),
	}
}

// Observe records an absolute cumulative value for a given label combo.
// On the first call for a label combo, Add(value) initialises the
// child. On subsequent calls, Add(value - last) moves it forward.
// If value < last (counter reset on the PG side), the child is deleted
// and re-added from the current value — equivalent to what
// prometheus-client-go's Counter would do on a reset from the
// server's perspective.
func (c *counterDelta) Observe(value float64, labelValues ...string) {
	key := strings.Join(labelValues, "\x00")

	c.mu.Lock()
	defer c.mu.Unlock()

	last, seen := c.last[key]
	switch {
	case !seen:
		c.vec.WithLabelValues(labelValues...).Add(value)
	case value < last:
		// PG counter reset. Drop the child entirely so its cumulative
		// state resets to 0, then Add the current value — this makes
		// the scrape emit show a decrease (from last to value), which
		// Prometheus correctly identifies as a counter reset and
		// handles in rate() / increase().
		c.vec.DeleteLabelValues(labelValues...)
		c.vec.WithLabelValues(labelValues...).Add(value)
	default:
		c.vec.WithLabelValues(labelValues...).Add(value - last)
	}
	c.last[key] = value
}

// Describe implements the part of prometheus.Collector a collector
// needs to wire through.
func (c *counterDelta) Describe(ch chan<- *prometheus.Desc) { c.vec.Describe(ch) }

// Collect yields every currently-tracked child metric to ch.
func (c *counterDelta) Collect(ch chan<- prometheus.Metric) { c.vec.Collect(ch) }

// counterFactory builds a *counterDelta scoped to a namespace + subsystem,
// matching the ergonomics of the previous NewDesc helper. Every collector
// wires one of these at construction time.
func counterFactory(subsystem string, labels []string) func(name, help string) *counterDelta {
	return func(name, help string) *counterDelta {
		return newCounterDelta(prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		}, labels))
	}
}

// counterFactoryIO is like counterFactory but uses the namespaceIO
// prefix for pg_statio_* collectors.
func counterFactoryIO(subsystem string, labels []string) func(name, help string) *counterDelta {
	return func(name, help string) *counterDelta {
		return newCounterDelta(prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceIO,
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		}, labels))
	}
}

// gaugeFactory mirrors counterFactory but returns a *prometheus.GaugeVec
// directly — gauges don't need delta tracking because the Gauge
// interface has Set(absolute) semantics natively.
func gaugeFactory(subsystem string, labels []string) func(name, help string) *prometheus.GaugeVec {
	return func(name, help string) *prometheus.GaugeVec {
		return prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		}, labels)
	}
}

// gaugeFactoryRawPg mirrors gaugeFactory for collectors that emit under
// the raw pg_* prefix (pg_replication_slots, pg_database_size) rather
// than pg_stat_* / pg_statio_*.
func gaugeFactoryRawPg(subsystem string, labels []string) func(name, help string) *prometheus.GaugeVec {
	return func(name, help string) *prometheus.GaugeVec {
		return prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespaceRawPg,
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		}, labels)
	}
}
