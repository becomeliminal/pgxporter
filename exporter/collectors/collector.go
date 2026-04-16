package collectors

import (
	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/logging"
	"github.com/prometheus/client_golang/prometheus"
)

var log = logging.NewLogger()

const (
	namespace   = "pg_stat"
	namespaceIO = "pg_statio"

	activitySubSystem    = "activity"
	locksSubSystem       = "locks"
	statementsSubSystem  = "statements"
	userTablesSubSystem  = "user_tables"
	userIndexesSubSystem = "user_indexes"
)

// Collector wraps the prometheus.Collector.
type Collector interface {
	prometheus.Collector
	// Scrape is used by our exporter to scrape data from postgres.
	Scrape(ch chan<- prometheus.Metric) error
}

// DefaultCollectors specifies the list of default collectors.
func DefaultCollectors(dbClients []*db.Client) []Collector {
	return []Collector{
		NewPgStatActivityCollector(dbClients),
		NewPgLocksCollector(dbClients),
		// Statement scrapes take way too long.
		// NewPgStatStatementsCollector(dbClients),
		NewPgStatUserTableCollector(dbClients),
		NewPgStatUserIndexesCollector(dbClients),
		NewPgStatIOUserTableCollector(dbClients),
		NewPgStatIOUserIndexesCollector(dbClients),
	}
}
