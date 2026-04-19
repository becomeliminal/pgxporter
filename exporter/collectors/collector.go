package collectors

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/logging"
)

var log = logging.NewLogger()

const (
	namespace      = "pg_stat"
	namespaceIO    = "pg_statio"
	namespaceRawPg = "pg"

	activitySubSystem         = "activity"
	archiverSubSystem         = "archiver"
	bgwriterSubSystem         = "bgwriter"
	checkpointerSubSystem     = "checkpointer"
	databaseSubSystem         = "database"
	locksSubSystem            = "locks"
	replicationSubSystem      = "replication"
	replicationSlotsSubSystem = "replication_slots"
	statementsSubSystem       = "statements"
	userTablesSubSystem       = "user_tables"
	userIndexesSubSystem      = "user_indexes"
	walReceiverSubSystem      = "wal_receiver"
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

// DefaultCollectors specifies the list of default collectors.
func DefaultCollectors(dbClients []*db.Client) []Collector {
	return []Collector{
		NewPgStatActivityCollector(dbClients),
		NewPgStatArchiverCollector(dbClients),
		NewPgStatBgwriterCollector(dbClients),
		NewPgStatCheckpointerCollector(dbClients),
		NewPgStatDatabaseCollector(dbClients),
		NewPgLocksCollector(dbClients),
		NewPgStatReplicationCollector(dbClients),
		NewPgReplicationSlotsCollector(dbClients),
		// Statement scrapes take way too long.
		// NewPgStatStatementsCollector(dbClients),
		NewPgStatUserTableCollector(dbClients),
		NewPgStatUserIndexesCollector(dbClients),
		NewPgStatIOUserTableCollector(dbClients),
		NewPgStatIOUserIndexesCollector(dbClients),
		NewPgStatWalReceiverCollector(dbClients),
	}
}
