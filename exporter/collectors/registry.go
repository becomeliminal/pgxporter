package collectors

import (
	"fmt"
	"sort"

	"github.com/becomeliminal/pgxporter/exporter/db"
)

// Collector names exposed via the enable/disable API. Matches
// postgres_exporter's convention (--collector.<name>) where practical.
const (
	CollectorActivity            = "activity"
	CollectorArchiver            = "archiver"
	CollectorBgwriter            = "bgwriter"
	CollectorCheckpointer        = "checkpointer"
	CollectorDatabase            = "database"
	CollectorDatabaseSize        = "database_size"
	CollectorIO                  = "io"
	CollectorLocks               = "locks"
	CollectorProgressAnalyze     = "progress_analyze"
	CollectorProgressBasebackup  = "progress_basebackup"
	CollectorProgressCluster     = "progress_cluster"
	CollectorProgressCopy        = "progress_copy"
	CollectorProgressCreateIndex = "progress_create_index"
	CollectorProgressVacuum      = "progress_vacuum"
	CollectorReplication         = "replication"
	CollectorReplicationSlots    = "replication_slots"
	CollectorSLRU                = "slru"
	CollectorSSL                 = "ssl"
	CollectorStatements          = "statements"
	CollectorUserIndexes         = "user_indexes"
	CollectorUserTables          = "user_tables"
	CollectorIOUserIndexes       = "io_user_indexes"
	CollectorIOUserTables        = "io_user_tables"
	CollectorWAL                 = "wal"
	CollectorWALReceiver         = "wal_receiver"
)

// collectorEntry pairs a stable collector name with its constructor and
// a default-enabled flag. Kept sorted alphabetically by Name to keep
// --help output predictable.
type collectorEntry struct {
	Name    string
	Default bool
	New     func([]*db.Client) Collector
}

// collectorRegistry is the single source of truth for which collectors
// exist and which are enabled by default. Every addition to
// DefaultCollectors should go here instead.
var collectorRegistry = []collectorEntry{
	{CollectorActivity, true, func(c []*db.Client) Collector { return NewPgStatActivityCollector(c) }},
	{CollectorArchiver, true, func(c []*db.Client) Collector { return NewPgStatArchiverCollector(c) }},
	{CollectorBgwriter, true, func(c []*db.Client) Collector { return NewPgStatBgwriterCollector(c) }},
	{CollectorCheckpointer, true, func(c []*db.Client) Collector { return NewPgStatCheckpointerCollector(c) }},
	{CollectorDatabase, true, func(c []*db.Client) Collector { return NewPgStatDatabaseCollector(c) }},
	{CollectorDatabaseSize, true, func(c []*db.Client) Collector { return NewPgDatabaseSizeCollector(c) }},
	{CollectorIO, true, func(c []*db.Client) Collector { return NewPgStatIOCollector(c) }},
	{CollectorIOUserIndexes, true, func(c []*db.Client) Collector { return NewPgStatIOUserIndexesCollector(c) }},
	{CollectorIOUserTables, true, func(c []*db.Client) Collector { return NewPgStatIOUserTableCollector(c) }},
	{CollectorLocks, true, func(c []*db.Client) Collector { return NewPgLocksCollector(c) }},
	{CollectorProgressAnalyze, true, func(c []*db.Client) Collector { return NewPgStatProgressAnalyzeCollector(c) }},
	{CollectorProgressBasebackup, true, func(c []*db.Client) Collector { return NewPgStatProgressBasebackupCollector(c) }},
	{CollectorProgressCluster, true, func(c []*db.Client) Collector { return NewPgStatProgressClusterCollector(c) }},
	{CollectorProgressCopy, true, func(c []*db.Client) Collector { return NewPgStatProgressCopyCollector(c) }},
	{CollectorProgressCreateIndex, true, func(c []*db.Client) Collector { return NewPgStatProgressCreateIndexCollector(c) }},
	{CollectorProgressVacuum, true, func(c []*db.Client) Collector { return NewPgStatProgressVacuumCollector(c) }},
	{CollectorReplication, true, func(c []*db.Client) Collector { return NewPgStatReplicationCollector(c) }},
	{CollectorReplicationSlots, true, func(c []*db.Client) Collector { return NewPgReplicationSlotsCollector(c) }},
	{CollectorSLRU, true, func(c []*db.Client) Collector { return NewPgStatSLRUCollector(c) }},
	{CollectorSSL, true, func(c []*db.Client) Collector { return NewPgStatSSLCollector(c) }},
	// Statements is off by default — pg_stat_statements queries are
	// expensive on busy clusters and the metric cardinality is high.
	// Users opt in via EnabledCollectors: []string{CollectorStatements}.
	{CollectorStatements, false, func(c []*db.Client) Collector { return NewPgStatStatementsCollector(c) }},
	{CollectorUserIndexes, true, func(c []*db.Client) Collector { return NewPgStatUserIndexesCollector(c) }},
	{CollectorUserTables, true, func(c []*db.Client) Collector { return NewPgStatUserTableCollector(c) }},
	{CollectorWAL, true, func(c []*db.Client) Collector { return NewPgStatWalCollector(c) }},
	{CollectorWALReceiver, true, func(c []*db.Client) Collector { return NewPgStatWalReceiverCollector(c) }},
}

// AvailableCollectors returns the sorted list of every collector name
// pgxporter can produce. Used for --help output and documentation.
func AvailableCollectors() []string {
	names := make([]string, 0, len(collectorRegistry))
	for _, e := range collectorRegistry {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names
}

// NamedCollector pairs a stable collector name with a constructed
// Collector. Returned by ResolveCollectors so the exporter can thread
// the name through to self-metrics (scrape duration, errors, metric
// cardinality) and log attributes without needing every Collector to
// implement a Name() method.
type NamedCollector struct {
	Name      string
	Collector Collector
}

// ResolveCollectors returns the collectors to run given enable/disable
// sets. Semantics:
//
//   - Both empty (nil or zero-length): every default-enabled collector.
//   - enabled non-empty: only those names are returned (overrides defaults).
//     This lets users opt in to pg_stat_statements without getting the
//     full default set.
//   - disabled always subtracts from the resolved set, even when enabled
//     is populated — disabled wins when a name appears in both.
//
// Unknown names (in either set) are returned as a non-fatal error so
// the caller can log and proceed: a typo in --no-collector.whatever
// shouldn't brick the exporter.
func ResolveCollectors(dbClients []*db.Client, enabled, disabled []string) ([]NamedCollector, error) {
	known := make(map[string]collectorEntry, len(collectorRegistry))
	for _, e := range collectorRegistry {
		known[e.Name] = e
	}

	var unknown []string
	enabledSet := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		if _, ok := known[name]; !ok {
			unknown = append(unknown, name)
			continue
		}
		enabledSet[name] = true
	}
	disabledSet := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		if _, ok := known[name]; !ok {
			unknown = append(unknown, name)
			continue
		}
		disabledSet[name] = true
	}

	out := make([]NamedCollector, 0, len(collectorRegistry))
	// When enabled is non-empty (even if every entry was unknown), the
	// user has asked for an explicit set: "only these". Falling back to
	// defaults because every requested name is unknown would surprise
	// them. Use the raw length here, not the validated set.
	explicitEnable := len(enabled) > 0
	for _, e := range collectorRegistry {
		include := e.Default
		if explicitEnable {
			include = enabledSet[e.Name]
		}
		if disabledSet[e.Name] {
			include = false
		}
		if include {
			out = append(out, NamedCollector{Name: e.Name, Collector: e.New(dbClients)})
		}
	}

	var err error
	if len(unknown) > 0 {
		err = fmt.Errorf("unknown collector name(s): %v (valid: %v)", unknown, AvailableCollectors())
	}
	return out, err
}
