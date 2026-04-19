package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatActivity aggregates pg_stat_activity by
// (datname, state, wait_event_type, backend_type) and returns the count
// plus max tx/query/backend durations in each bucket. Low-cardinality
// labels only — query text and pids are intentionally not exposed.
//
// pg_stat_activity is cluster-wide (not per-database); the database
// label identifies the connection this exporter is attached to, and
// datname identifies the database each backend is connected to.
func (db *Client) SelectPgStatActivity(ctx context.Context) ([]*model.PgStatActivity, error) {
	rows := []*model.PgStatActivity{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(datname, '') AS datname,
		COALESCE(state, 'disabled') AS state,
		COALESCE(wait_event_type, 'none') AS wait_event_type,
		COALESCE(backend_type, 'unknown') AS backend_type,
		count(*) AS count,
		COALESCE(MAX(EXTRACT(EPOCH FROM now() - xact_start))::float8, 0) AS max_tx_duration_seconds,
		COALESCE(MAX(EXTRACT(EPOCH FROM now() - query_start))::float8, 0) AS max_query_duration_seconds,
		COALESCE(MAX(EXTRACT(EPOCH FROM now() - backend_start))::float8, 0) AS max_backend_age_seconds
	FROM pg_stat_activity
	GROUP BY datname, state, wait_event_type, backend_type`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
