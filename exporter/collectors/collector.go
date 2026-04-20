package collectors

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db"
)

// Prometheus metric namespace prefixes. Exposed as vars (not consts) so
// that SetMetricPrefix can flip namespace at startup — useful for the
// postgres_exporter compatibility mode where community Grafana
// dashboards expect "pg_*" metric names instead of our "pg_stat_*"
// default. See SetMetricPrefix below for the full contract.
var (
	namespace      = "pg_stat"
	namespaceIO    = "pg_statio"
	namespaceRawPg = "pg"
)

const (
	activitySubSystem            = "activity"
	archiverSubSystem            = "archiver"
	bgwriterSubSystem            = "bgwriter"
	checkpointerSubSystem        = "checkpointer"
	databaseSubSystem            = "database"
	ioSubSystem                  = "io"
	locksSubSystem               = "locks"
	progressAnalyzeSubSystem     = "progress_analyze"
	progressBasebackupSubSystem  = "progress_basebackup"
	progressClusterSubSystem     = "progress_cluster"
	progressCopySubSystem        = "progress_copy"
	progressCreateIndexSubSystem = "progress_create_index"
	progressVacuumSubSystem      = "progress_vacuum"
	replicationSubSystem         = "replication"
	replicationSlotsSubSystem    = "replication_slots"
	slruSubSystem                = "slru"
	statementsSubSystem          = "statements"
	userTablesSubSystem          = "user_tables"
	userIndexesSubSystem         = "user_indexes"
	walSubSystem                 = "wal"
	walReceiverSubSystem         = "wal_receiver"
)

// Collector is a scraper for one Postgres statistics view.
//
// It is intentionally NOT a [prometheus.Collector]: collectors in this
// package are always driven through [exporter.Exporter], which aggregates
// them and calls Scrape directly. Registering a bare collector with
// prometheus.MustRegister would previously deadlock because Collect held
// a write lock and Scrape re-acquired it from goroutines. Callers who
// genuinely want to register a single collector with Prometheus must
// wrap it themselves.
type Collector interface {
	// Describe emits the Prometheus descriptors this collector exports.
	Describe(ch chan<- *prometheus.Desc)
	// Scrape queries postgres and emits one metric per row on ch.
	// ctx carries the per-scrape deadline set by [exporter.Exporter];
	// implementations MUST pass it to every DB call and honour cancellation
	// so a pathological query cannot hang the whole scrape cycle.
	Scrape(ctx context.Context, ch chan<- prometheus.Metric) error
}

// MetricPrefix is the top-level namespace for pg_stat_* metric families.
// "pg_stat" (the default) produces names like pg_stat_database_xact_commit_total;
// "pg" produces pg_database_xact_commit_total — the naming convention
// postgres_exporter's dashboards expect. Other values are allowed but
// aren't covered by any known community dashboard set.
type MetricPrefix string

const (
	// MetricPrefixPgStat is the default — names like pg_stat_database_xact_commit.
	MetricPrefixPgStat MetricPrefix = "pg_stat"
	// MetricPrefixPg matches postgres_exporter's prefix, making its
	// community Grafana dashboards work against pgxporter.
	MetricPrefixPg MetricPrefix = "pg"
)

// SetMetricPrefix switches the "pg_stat" namespace to a user-chosen
// value BEFORE any collector is constructed. Intended use:
//
//	collectors.SetMetricPrefix(collectors.MetricPrefixPg)
//	exp, err := exporter.New(ctx, opts)  // collectors pick up the new prefix
//
// Calling this AFTER collectors have been constructed has no effect
// on those existing descriptors — the prefix is baked into each desc
// at NewXxxCollector time. This is a library-level switch (one flip
// per process), not a per-collector override.
//
// namespaceIO (pg_statio_*) and the raw pg_* namespace used by
// pg_replication_slots and pg_database_size are unaffected: they're
// already PG-native names and don't need remapping.
func SetMetricPrefix(p MetricPrefix) {
	if p == "" {
		p = MetricPrefixPgStat
	}
	namespace = string(p)
}

// DefaultCollectors returns every default-enabled collector. Equivalent
// to ResolveCollectors(dbClients, nil, nil) minus names and the error
// return. Kept for backward compatibility — new code should prefer
// ResolveCollectors with explicit enable/disable semantics.
func DefaultCollectors(dbClients []*db.Client) []Collector {
	named, _ := ResolveCollectors(dbClients, nil, nil)
	out := make([]Collector, len(named))
	for i, n := range named {
		out[i] = n.Collector
	}
	return out
}
