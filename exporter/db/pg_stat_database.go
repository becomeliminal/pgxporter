package db

import (
	"context"
	"strings"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatDatabase selects cluster-health stats per database.
// Version-gated columns are only projected on servers new enough to expose
// them; on older servers the corresponding struct fields remain zero-valued
// (pgtype.Valid == false) and the collector skips them.
func (db *Client) SelectPgStatDatabase(ctx context.Context) ([]*model.PgStatDatabase, error) {
	rows := []*model.PgStatDatabase{}
	if err := db.Select(ctx, &rows, db.sqlSelectPgStatDatabase()); err != nil {
		return nil, err
	}
	return rows, nil
}

// sqlSelectPgStatDatabase composes the SELECT based on the cached server
// version. Missing columns simply aren't projected.
func (db *Client) sqlSelectPgStatDatabase() string {
	cols := []string{
		"current_database() as database",
		"datid",
		"datname",
		"numbackends",
		"xact_commit",
		"xact_rollback",
		"blks_read",
		"blks_hit",
		"tup_returned",
		"tup_fetched",
		"tup_inserted",
		"tup_updated",
		"tup_deleted",
		"conflicts",
		"temp_files",
		"temp_bytes",
		"deadlocks",
		"blk_read_time",
		"blk_write_time",
		"stats_reset",
	}
	if db.AtLeast(12, 0) {
		cols = append(cols, "checksum_failures", "checksum_last_failure")
	}
	if db.AtLeast(14, 0) {
		cols = append(cols,
			"session_time",
			"active_time",
			"idle_in_transaction_time",
			"sessions",
			"sessions_abandoned",
			"sessions_fatal",
			"sessions_killed",
		)
	}
	// Exclude rows for template dbs + the background writer pseudo-db (datid=0)
	// — callers rarely want those and they pollute the per-datname dashboards.
	return "SELECT " + strings.Join(cols, ", ") +
		" FROM pg_stat_database WHERE datname IS NOT NULL AND datname NOT LIKE 'template%'"
}
