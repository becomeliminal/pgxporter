package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgLocks aggregates pg_locks by (datname, mode, locktype, granted).
// Drops the legacy CROSS JOIN pattern (which emitted zeros for every
// possible mode) — we only emit rows for lock modes actually observed.
func (db *Client) SelectPgLocks(ctx context.Context) ([]*model.PgLock, error) {
	rows := []*model.PgLock{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(pg_database.datname, '') AS datname,
		mode,
		locktype,
		granted,
		count(*) AS count
	FROM pg_locks
	LEFT JOIN pg_database ON pg_database.oid = pg_locks.database
	GROUP BY pg_database.datname, mode, locktype, granted`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}

// SelectPgLocksBlockingSummary returns a scalar row summarising the
// blocking-chain state across the cluster: the number of backends
// currently waiting on another backend, and the total number of
// (blocked, blocker) edges. Uses pg_blocking_pids() which is O(N²) in
// backend count — callers wanting per-relation or per-chain detail
// should query pg_locks directly.
func (db *Client) SelectPgLocksBlockingSummary(ctx context.Context) ([]*model.PgLocksBlockingSummary, error) {
	rows := []*model.PgLocksBlockingSummary{}
	const sql = `SELECT
		current_database() AS database,
		COUNT(*) FILTER (WHERE cardinality(pg_blocking_pids(pid)) > 0)::bigint AS blocked_backends,
		COALESCE(SUM(cardinality(pg_blocking_pids(pid))), 0)::bigint AS blocker_edges
	FROM pg_stat_activity
	WHERE state IS NOT NULL`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
