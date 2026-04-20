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

// PgStatDatabaseCollector collects from pg_stat_database — the single most
// important cluster-health view. Gives you transaction rates, cache hit
// ratios, deadlocks, and (on PG 14+) per-session timing.
type PgStatDatabaseCollector struct {
	dbClients []*db.Client

	numBackends  *prometheus.Desc
	xactCommit   *prometheus.Desc
	xactRollback *prometheus.Desc
	blksRead     *prometheus.Desc
	blksHit      *prometheus.Desc
	tupReturned  *prometheus.Desc
	tupFetched   *prometheus.Desc
	tupInserted  *prometheus.Desc
	tupUpdated   *prometheus.Desc
	tupDeleted   *prometheus.Desc
	conflicts    *prometheus.Desc
	tempFiles    *prometheus.Desc
	tempBytes    *prometheus.Desc
	deadlocks    *prometheus.Desc
	blkReadTime  *prometheus.Desc // seconds
	blkWriteTime *prometheus.Desc // seconds
	statsReset   *prometheus.Desc

	// PG 12+
	checksumFailures    *prometheus.Desc
	checksumLastFailure *prometheus.Desc

	// PG 14+
	sessionTime           *prometheus.Desc // seconds
	activeTime            *prometheus.Desc // seconds
	idleInTransactionTime *prometheus.Desc // seconds
	sessions              *prometheus.Desc
	sessionsAbandoned     *prometheus.Desc
	sessionsFatal         *prometheus.Desc
	sessionsKilled        *prometheus.Desc
}

// NewPgStatDatabaseCollector instantiates and returns a new PgStatDatabaseCollector.
func NewPgStatDatabaseCollector(dbClients []*db.Client) *PgStatDatabaseCollector {
	labels := []string{"database", "datname"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, databaseSubSystem, name), help, labels, nil)
	}
	return &PgStatDatabaseCollector{
		dbClients: dbClients,

		numBackends:  desc("numbackends", "Number of backends currently connected to this database"),
		xactCommit:   desc("xact_commit_total", "Transactions committed in this database"),
		xactRollback: desc("xact_rollback_total", "Transactions rolled back in this database"),
		blksRead:     desc("blks_read_total", "Disk blocks read in this database"),
		blksHit:      desc("blks_hit_total", "Buffer cache hits in this database"),
		tupReturned:  desc("tup_returned_total", "Live rows fetched by sequential scans + index entries returned by index scans"),
		tupFetched:   desc("tup_fetched_total", "Live rows fetched by index scans"),
		tupInserted:  desc("tup_inserted_total", "Rows inserted by queries in this database"),
		tupUpdated:   desc("tup_updated_total", "Rows updated by queries in this database"),
		tupDeleted:   desc("tup_deleted_total", "Rows deleted by queries in this database"),
		conflicts:    desc("conflicts_total", "Queries canceled due to conflicts with recovery on standbys"),
		tempFiles:    desc("temp_files_total", "Temporary files created by queries in this database"),
		tempBytes:    desc("temp_bytes_total", "Bytes written to temporary files in this database"),
		deadlocks:    desc("deadlocks_total", "Deadlocks detected in this database"),
		blkReadTime:  desc("blk_read_time_seconds_total", "Time spent reading data blocks by backends in this database (seconds)"),
		blkWriteTime: desc("blk_write_time_seconds_total", "Time spent writing data blocks by backends in this database (seconds)"),
		statsReset:   desc("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),

		checksumFailures:    desc("checksum_failures_total", "Data page checksum failures detected in this database (PG 12+)"),
		checksumLastFailure: desc("checksum_last_failure_timestamp_seconds", "Unix time of the last data page checksum failure (PG 12+)"),

		sessionTime:           desc("session_time_seconds_total", "Total session time in this database (seconds, PG 14+)"),
		activeTime:            desc("active_time_seconds_total", "Total active-query time in this database (seconds, PG 14+)"),
		idleInTransactionTime: desc("idle_in_transaction_time_seconds_total", "Total idle-in-transaction time in this database (seconds, PG 14+)"),
		sessions:              desc("sessions_total", "Sessions established in this database (PG 14+)"),
		sessionsAbandoned:     desc("sessions_abandoned_total", "Sessions abandoned due to connection loss (PG 14+)"),
		sessionsFatal:         desc("sessions_fatal_total", "Sessions terminated by fatal errors (PG 14+)"),
		sessionsKilled:        desc("sessions_killed_total", "Sessions terminated by operator intervention (PG 14+)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatDatabaseCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.numBackends
	ch <- c.xactCommit
	ch <- c.xactRollback
	ch <- c.blksRead
	ch <- c.blksHit
	ch <- c.tupReturned
	ch <- c.tupFetched
	ch <- c.tupInserted
	ch <- c.tupUpdated
	ch <- c.tupDeleted
	ch <- c.conflicts
	ch <- c.tempFiles
	ch <- c.tempBytes
	ch <- c.deadlocks
	ch <- c.blkReadTime
	ch <- c.blkWriteTime
	ch <- c.statsReset
	ch <- c.checksumFailures
	ch <- c.checksumLastFailure
	ch <- c.sessionTime
	ch <- c.activeTime
	ch <- c.idleInTransactionTime
	ch <- c.sessions
	ch <- c.sessionsAbandoned
	ch <- c.sessionsFatal
	ch <- c.sessionsKilled
}

// Scrape implements our Scraper interface.
func (c *PgStatDatabaseCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(gctx, dbClient, ch) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatDatabaseCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatDatabase(ctx)
	if err != nil {
		return fmt.Errorf("database stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

// emit turns scanned pg_stat_database rows into metrics, skipping NULL
// columns. Separated from scrape for unit-test coverage.
func (c *PgStatDatabaseCollector) emit(stats []*model.PgStatDatabase, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		database, datname := stat.Database.String, stat.DatName.String
		emitInt := func(desc *prometheus.Desc, valueType prometheus.ValueType, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, valueType, float64(v.Int64), database, datname)
			}
		}
		// Postgres exposes blk_read_time / blk_write_time in milliseconds; we
		// publish seconds to match the Prometheus convention of base SI units.
		emitMillisAsSecs := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, v.Float64/1000.0, database, datname)
			}
		}
		// PG 14+ session-time columns are already in milliseconds per the docs.
		emitMillisAsSecsFloat := emitMillisAsSecs

		emitTime := func(desc *prometheus.Desc, v pgtype.Timestamptz) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Time.Unix()), database, datname)
			}
		}

		emitInt(c.numBackends, prometheus.GaugeValue, stat.NumBackends)
		emitInt(c.xactCommit, prometheus.CounterValue, stat.XactCommit)
		emitInt(c.xactRollback, prometheus.CounterValue, stat.XactRollback)
		emitInt(c.blksRead, prometheus.CounterValue, stat.BlksRead)
		emitInt(c.blksHit, prometheus.CounterValue, stat.BlksHit)
		emitInt(c.tupReturned, prometheus.CounterValue, stat.TupReturned)
		emitInt(c.tupFetched, prometheus.CounterValue, stat.TupFetched)
		emitInt(c.tupInserted, prometheus.CounterValue, stat.TupInserted)
		emitInt(c.tupUpdated, prometheus.CounterValue, stat.TupUpdated)
		emitInt(c.tupDeleted, prometheus.CounterValue, stat.TupDeleted)
		emitInt(c.conflicts, prometheus.CounterValue, stat.Conflicts)
		emitInt(c.tempFiles, prometheus.CounterValue, stat.TempFiles)
		emitInt(c.tempBytes, prometheus.CounterValue, stat.TempBytes)
		emitInt(c.deadlocks, prometheus.CounterValue, stat.Deadlocks)
		emitMillisAsSecs(c.blkReadTime, stat.BlkReadTime)
		emitMillisAsSecs(c.blkWriteTime, stat.BlkWriteTime)
		emitTime(c.statsReset, stat.StatsReset)

		// PG 12+
		emitInt(c.checksumFailures, prometheus.CounterValue, stat.ChecksumFailures)
		emitTime(c.checksumLastFailure, stat.ChecksumLastFailure)

		// PG 14+
		emitMillisAsSecsFloat(c.sessionTime, stat.SessionTime)
		emitMillisAsSecsFloat(c.activeTime, stat.ActiveTime)
		emitMillisAsSecsFloat(c.idleInTransactionTime, stat.IdleInTransactionTime)
		emitInt(c.sessions, prometheus.CounterValue, stat.Sessions)
		emitInt(c.sessionsAbandoned, prometheus.CounterValue, stat.SessionsAbandoned)
		emitInt(c.sessionsFatal, prometheus.CounterValue, stat.SessionsFatal)
		emitInt(c.sessionsKilled, prometheus.CounterValue, stat.SessionsKilled)
	}
}
