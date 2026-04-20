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

	numBackends  *prometheus.GaugeVec
	xactCommit   *counterDelta
	xactRollback *counterDelta
	blksRead     *counterDelta
	blksHit      *counterDelta
	tupReturned  *counterDelta
	tupFetched   *counterDelta
	tupInserted  *counterDelta
	tupUpdated   *counterDelta
	tupDeleted   *counterDelta
	conflicts    *counterDelta
	tempFiles    *counterDelta
	tempBytes    *counterDelta
	deadlocks    *counterDelta
	blkReadTime  *counterDelta // seconds
	blkWriteTime *counterDelta // seconds
	statsReset   *prometheus.GaugeVec

	// PG 12+
	checksumFailures    *counterDelta
	checksumLastFailure *prometheus.GaugeVec

	// PG 14+
	sessionTime           *counterDelta // seconds
	activeTime            *counterDelta // seconds
	idleInTransactionTime *counterDelta // seconds
	sessions              *counterDelta
	sessionsAbandoned     *counterDelta
	sessionsFatal         *counterDelta
	sessionsKilled        *counterDelta
}

// NewPgStatDatabaseCollector instantiates and returns a new PgStatDatabaseCollector.
func NewPgStatDatabaseCollector(dbClients []*db.Client) *PgStatDatabaseCollector {
	labels := []string{"database", "datname"}
	counter := counterFactory(databaseSubSystem, labels)
	gauge := gaugeFactory(databaseSubSystem, labels)
	return &PgStatDatabaseCollector{
		dbClients: dbClients,

		numBackends:  gauge("numbackends", "Number of backends currently connected to this database"),
		xactCommit:   counter("xact_commit_total", "Transactions committed in this database"),
		xactRollback: counter("xact_rollback_total", "Transactions rolled back in this database"),
		blksRead:     counter("blks_read_total", "Disk blocks read in this database"),
		blksHit:      counter("blks_hit_total", "Buffer cache hits in this database"),
		tupReturned:  counter("tup_returned_total", "Live rows fetched by sequential scans + index entries returned by index scans"),
		tupFetched:   counter("tup_fetched_total", "Live rows fetched by index scans"),
		tupInserted:  counter("tup_inserted_total", "Rows inserted by queries in this database"),
		tupUpdated:   counter("tup_updated_total", "Rows updated by queries in this database"),
		tupDeleted:   counter("tup_deleted_total", "Rows deleted by queries in this database"),
		conflicts:    counter("conflicts_total", "Queries canceled due to conflicts with recovery on standbys"),
		tempFiles:    counter("temp_files_total", "Temporary files created by queries in this database"),
		tempBytes:    counter("temp_bytes_total", "Bytes written to temporary files in this database"),
		deadlocks:    counter("deadlocks_total", "Deadlocks detected in this database"),
		blkReadTime:  counter("blk_read_time_seconds_total", "Time spent reading data blocks by backends in this database (seconds)"),
		blkWriteTime: counter("blk_write_time_seconds_total", "Time spent writing data blocks by backends in this database (seconds)"),
		statsReset:   gauge("stats_reset_timestamp_seconds", "Unix time at which these stats were last reset"),

		checksumFailures:    counter("checksum_failures_total", "Data page checksum failures detected in this database (PG 12+)"),
		checksumLastFailure: gauge("checksum_last_failure_timestamp_seconds", "Unix time of the last data page checksum failure (PG 12+)"),

		sessionTime:           counter("session_time_seconds_total", "Total session time in this database (seconds, PG 14+)"),
		activeTime:            counter("active_time_seconds_total", "Total active-query time in this database (seconds, PG 14+)"),
		idleInTransactionTime: counter("idle_in_transaction_time_seconds_total", "Total idle-in-transaction time in this database (seconds, PG 14+)"),
		sessions:              counter("sessions_total", "Sessions established in this database (PG 14+)"),
		sessionsAbandoned:     counter("sessions_abandoned_total", "Sessions abandoned due to connection loss (PG 14+)"),
		sessionsFatal:         counter("sessions_fatal_total", "Sessions terminated by fatal errors (PG 14+)"),
		sessionsKilled:        counter("sessions_killed_total", "Sessions terminated by operator intervention (PG 14+)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatDatabaseCollector) Describe(ch chan<- *prometheus.Desc) {
	c.numBackends.Describe(ch)
	c.xactCommit.Describe(ch)
	c.xactRollback.Describe(ch)
	c.blksRead.Describe(ch)
	c.blksHit.Describe(ch)
	c.tupReturned.Describe(ch)
	c.tupFetched.Describe(ch)
	c.tupInserted.Describe(ch)
	c.tupUpdated.Describe(ch)
	c.tupDeleted.Describe(ch)
	c.conflicts.Describe(ch)
	c.tempFiles.Describe(ch)
	c.tempBytes.Describe(ch)
	c.deadlocks.Describe(ch)
	c.blkReadTime.Describe(ch)
	c.blkWriteTime.Describe(ch)
	c.statsReset.Describe(ch)
	c.checksumFailures.Describe(ch)
	c.checksumLastFailure.Describe(ch)
	c.sessionTime.Describe(ch)
	c.activeTime.Describe(ch)
	c.idleInTransactionTime.Describe(ch)
	c.sessions.Describe(ch)
	c.sessionsAbandoned.Describe(ch)
	c.sessionsFatal.Describe(ch)
	c.sessionsKilled.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatDatabaseCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(gctx, dbClient) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatDatabaseCollector) collectInto(ch chan<- prometheus.Metric) {
	c.numBackends.Collect(ch)
	c.xactCommit.Collect(ch)
	c.xactRollback.Collect(ch)
	c.blksRead.Collect(ch)
	c.blksHit.Collect(ch)
	c.tupReturned.Collect(ch)
	c.tupFetched.Collect(ch)
	c.tupInserted.Collect(ch)
	c.tupUpdated.Collect(ch)
	c.tupDeleted.Collect(ch)
	c.conflicts.Collect(ch)
	c.tempFiles.Collect(ch)
	c.tempBytes.Collect(ch)
	c.deadlocks.Collect(ch)
	c.blkReadTime.Collect(ch)
	c.blkWriteTime.Collect(ch)
	c.statsReset.Collect(ch)
	c.checksumFailures.Collect(ch)
	c.checksumLastFailure.Collect(ch)
	c.sessionTime.Collect(ch)
	c.activeTime.Collect(ch)
	c.idleInTransactionTime.Collect(ch)
	c.sessions.Collect(ch)
	c.sessionsAbandoned.Collect(ch)
	c.sessionsFatal.Collect(ch)
	c.sessionsKilled.Collect(ch)
}

func (c *PgStatDatabaseCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatDatabase(ctx)
	if err != nil {
		return fmt.Errorf("database stats: %w", err)
	}
	c.emit(stats)
	return nil
}

// emit turns scanned pg_stat_database rows into metrics, skipping NULL
// columns. Separated from scrape for unit-test coverage.
func (c *PgStatDatabaseCollector) emit(stats []*model.PgStatDatabase) {
	for _, stat := range stats {
		database, datname := stat.Database.String, stat.DatName.String
		emitGauge := func(vec *prometheus.GaugeVec, v pgtype.Int8) {
			if v.Valid {
				vec.WithLabelValues(database, datname).Set(float64(v.Int64))
			}
		}
		emitCounter := func(cd *counterDelta, v pgtype.Int8) {
			if v.Valid {
				cd.Observe(float64(v.Int64), database, datname)
			}
		}
		// Postgres exposes blk_read_time / blk_write_time in milliseconds; we
		// publish seconds to match the Prometheus convention of base SI units.
		emitMillisAsSecs := func(cd *counterDelta, v pgtype.Float8) {
			if v.Valid {
				cd.Observe(v.Float64/1000.0, database, datname)
			}
		}
		// PG 14+ session-time columns are already in milliseconds per the docs.
		emitMillisAsSecsFloat := emitMillisAsSecs

		emitTime := func(vec *prometheus.GaugeVec, v pgtype.Timestamptz) {
			if v.Valid {
				vec.WithLabelValues(database, datname).Set(float64(v.Time.Unix()))
			}
		}

		emitGauge(c.numBackends, stat.NumBackends)
		emitCounter(c.xactCommit, stat.XactCommit)
		emitCounter(c.xactRollback, stat.XactRollback)
		emitCounter(c.blksRead, stat.BlksRead)
		emitCounter(c.blksHit, stat.BlksHit)
		emitCounter(c.tupReturned, stat.TupReturned)
		emitCounter(c.tupFetched, stat.TupFetched)
		emitCounter(c.tupInserted, stat.TupInserted)
		emitCounter(c.tupUpdated, stat.TupUpdated)
		emitCounter(c.tupDeleted, stat.TupDeleted)
		emitCounter(c.conflicts, stat.Conflicts)
		emitCounter(c.tempFiles, stat.TempFiles)
		emitCounter(c.tempBytes, stat.TempBytes)
		emitCounter(c.deadlocks, stat.Deadlocks)
		emitMillisAsSecs(c.blkReadTime, stat.BlkReadTime)
		emitMillisAsSecs(c.blkWriteTime, stat.BlkWriteTime)
		emitTime(c.statsReset, stat.StatsReset)

		// PG 12+
		emitCounter(c.checksumFailures, stat.ChecksumFailures)
		emitTime(c.checksumLastFailure, stat.ChecksumLastFailure)

		// PG 14+
		emitMillisAsSecsFloat(c.sessionTime, stat.SessionTime)
		emitMillisAsSecsFloat(c.activeTime, stat.ActiveTime)
		emitMillisAsSecsFloat(c.idleInTransactionTime, stat.IdleInTransactionTime)
		emitCounter(c.sessions, stat.Sessions)
		emitCounter(c.sessionsAbandoned, stat.SessionsAbandoned)
		emitCounter(c.sessionsFatal, stat.SessionsFatal)
		emitCounter(c.sessionsKilled, stat.SessionsKilled)
	}
}
