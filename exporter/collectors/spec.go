package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
)

// CollectorSpec declaratively describes a collector: its SQL, which
// columns are labels, and which are metric values. Runtime-loadable —
// the default registry is hand-written Go for control and clarity, but
// operators with custom metrics can point pgxporter at a YAML file of
// CollectorSpecs (see the --extend.config ticket) and get a collector
// without forking or recompiling.
//
// This is a much cleaner replacement for postgres_exporter's deprecated
// queries.yaml pattern: strongly typed, version-gated, label-aware.
type CollectorSpec struct {
	// Subsystem becomes the second component of each metric name:
	//   <namespace>_<Subsystem>_<Metric.Name>
	// Typically the view name without the pg_stat_ prefix, e.g. "archiver".
	Subsystem string

	// Namespace overrides the default "pg_stat" namespace. Rarely needed
	// (pg_database_size uses "pg" because the underlying function isn't
	// a pg_stat_* view).
	Namespace string

	// SQL is the SELECT statement. It MUST return a "database" column
	// first (we enforce this so internal dashboards can rely on a
	// consistent cross-collector label); all other columns are either
	// labels (listed in Labels) or metric values.
	SQL string

	// MinPGVersion gates the collector by PG major version. Zero means
	// no minimum. Example: {13, 0} for pg_stat_slru.
	MinPGVersion [2]int

	// Labels are the SQL column names used as Prometheus labels,
	// excluding "database" (which is always emitted as the first label).
	Labels []string

	// Metrics are the SQL columns emitted as metric values. Each
	// MetricSpec becomes one prometheus.Metric.
	Metrics []MetricSpec
}

// MetricSpec describes one metric emitted from a CollectorSpec row.
type MetricSpec struct {
	// Name is the metric name suffix — the third component of the FQN:
	//   <Namespace>_<Subsystem>_<Name>
	Name string
	// Help is the descriptor help text (user-facing).
	Help string
	// Type is CounterValue or GaugeValue. Default GaugeValue.
	Type prometheus.ValueType
	// Column is the SQL column to read the value from.
	Column string
	// Scale is an optional multiplier applied to the scanned value
	// before emission. Example: pg_stat_wal's *_time columns come back
	// as milliseconds — set Scale: 0.001 to convert to seconds.
	// Zero/unset means no scaling.
	Scale float64
	// TimestampAsUnix, if true, reads the column as a timestamptz and
	// emits time.Time.Unix() (useful for stats_reset columns).
	TimestampAsUnix bool
}

// SpecCollector is a generic collector driven by a CollectorSpec. Its
// Scrape runs the spec's SQL on every dbClient, then emits one metric
// per (row, MetricSpec). NULLs are skipped (consistent with hand-written
// collectors).
type SpecCollector struct {
	spec      CollectorSpec
	dbClients []*db.Client

	descs       []*prometheus.Desc // one per MetricSpec, same order as spec.Metrics
	metricTypes []prometheus.ValueType
}

// NewSpecCollector builds a SpecCollector. Panics if the spec is
// invalid (missing Subsystem, no Metrics, etc.) — registration-time
// misconfiguration is a programmer error, not a runtime condition.
func NewSpecCollector(spec CollectorSpec, dbClients []*db.Client) *SpecCollector {
	if spec.Subsystem == "" {
		panic("CollectorSpec.Subsystem is required")
	}
	if spec.SQL == "" {
		panic("CollectorSpec.SQL is required")
	}
	if len(spec.Metrics) == 0 {
		panic("CollectorSpec.Metrics must contain at least one MetricSpec")
	}
	ns := spec.Namespace
	if ns == "" {
		ns = namespace
	}
	// Labels always include "database" first so downstream dashboards
	// can group cross-collector by db without special-casing.
	labels := append([]string{"database"}, spec.Labels...)

	descs := make([]*prometheus.Desc, len(spec.Metrics))
	types := make([]prometheus.ValueType, len(spec.Metrics))
	for i, m := range spec.Metrics {
		if m.Name == "" || m.Column == "" {
			panic(fmt.Sprintf("MetricSpec[%d]: Name and Column are required", i))
		}
		descs[i] = prometheus.NewDesc(
			prometheus.BuildFQName(ns, spec.Subsystem, m.Name),
			m.Help, labels, nil,
		)
		types[i] = m.Type
		if types[i] == 0 {
			types[i] = prometheus.GaugeValue
		}
	}
	return &SpecCollector{
		spec:        spec,
		dbClients:   dbClients,
		descs:       descs,
		metricTypes: types,
	}
}

// Describe implements prometheus.Collector.
func (c *SpecCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descs {
		ch <- d
	}
}

// Scrape implements the collectors.Collector interface. Version gate is
// applied per-dbClient so a single pre-version server doesn't silence
// the whole collector in a heterogeneous deployment.
func (c *SpecCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	start := time.Now()
	defer func() {
		log.Infof("spec(%s) scrape took %dms", c.spec.Subsystem, time.Since(start).Milliseconds())
	}()
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(gctx, dbClient, ch) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("spec(%s): %w", c.spec.Subsystem, err)
	}
	return nil
}

func (c *SpecCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	if c.spec.MinPGVersion[0] > 0 && !dbClient.AtLeast(c.spec.MinPGVersion[0], c.spec.MinPGVersion[1]) {
		return nil
	}
	rows, err := dbClient.Query(ctx, c.spec.SQL)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	// Build a column-name → index map once per scrape so each row lookup
	// is O(1).
	fieldDescs := rows.FieldDescriptions()
	colIdx := make(map[string]int, len(fieldDescs))
	for i, fd := range fieldDescs {
		colIdx[fd.Name] = i
	}
	if _, ok := colIdx["database"]; !ok {
		return fmt.Errorf("spec(%s): SELECT must return a 'database' column", c.spec.Subsystem)
	}
	for _, lbl := range c.spec.Labels {
		if _, ok := colIdx[lbl]; !ok {
			return fmt.Errorf("spec(%s): label column %q missing from SELECT", c.spec.Subsystem, lbl)
		}
	}
	for _, m := range c.spec.Metrics {
		if _, ok := colIdx[m.Column]; !ok {
			return fmt.Errorf("spec(%s): metric column %q missing from SELECT", c.spec.Subsystem, m.Column)
		}
	}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return fmt.Errorf("row values: %w", err)
		}
		// Assemble labels: database first, then the declared label order.
		labels := make([]string, 0, 1+len(c.spec.Labels))
		labels = append(labels, coerceString(values[colIdx["database"]]))
		for _, lbl := range c.spec.Labels {
			labels = append(labels, coerceString(values[colIdx[lbl]]))
		}
		// Emit each metric from this row.
		for i, m := range c.spec.Metrics {
			raw := values[colIdx[m.Column]]
			if raw == nil {
				continue
			}
			val, ok := coerceFloat(raw, m.TimestampAsUnix)
			if !ok {
				continue
			}
			if m.Scale != 0 {
				val *= m.Scale
			}
			vtype := c.metricTypes[i]
			if m.TimestampAsUnix {
				vtype = prometheus.GaugeValue
			}
			ch <- prometheus.MustNewConstMetric(c.descs[i], vtype, val, labels...)
		}
	}
	return rows.Err()
}

// coerceString turns a scanned value into a label string. pgx returns
// string, int, bool, time.Time, []byte, etc. depending on the column
// type; empty string is the safe default for a missing value.
func coerceString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprint(x)
	}
}

// coerceFloat converts a scanned value to a float64 for metric emission.
// Handles the common numeric types + time.Time (as Unix seconds when
// TimestampAsUnix is true).
func coerceFloat(v any, tsAsUnix bool) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case time.Time:
		if tsAsUnix {
			return float64(x.Unix()), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// assert that SpecCollector satisfies the Collector interface.
var _ Collector = (*SpecCollector)(nil)

// pgx.Rows is referenced via db.Client.Query's return type; this ensures
// the import isn't flagged as unused even if the compiler infers the
// type elsewhere.
var _ pgx.Rows
