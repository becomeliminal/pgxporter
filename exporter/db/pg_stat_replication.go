package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatReplication selects per-replica replication progress. Returns
// zero rows on a primary with no replicas connected, and typically zero rows
// on a standby (unless it's a cascading primary). The LSN-diff columns short-
// circuit via CASE WHEN pg_is_in_recovery() so the query doesn't fail on a
// standby where pg_current_wal_lsn() isn't callable.
func (db *Client) SelectPgStatReplication(ctx context.Context) ([]*model.PgStatReplication, error) {
	rows := []*model.PgStatReplication{}
	const sql = `SELECT
		current_database() AS database,
		pid,
		application_name,
		client_addr::text AS client_addr,
		state,
		sync_state,
		CASE WHEN pg_is_in_recovery() THEN NULL
		     ELSE pg_wal_lsn_diff(pg_current_wal_lsn(), sent_lsn)::float8 END AS sent_lag_bytes,
		CASE WHEN pg_is_in_recovery() THEN NULL
		     ELSE pg_wal_lsn_diff(pg_current_wal_lsn(), write_lsn)::float8 END AS write_lag_bytes,
		CASE WHEN pg_is_in_recovery() THEN NULL
		     ELSE pg_wal_lsn_diff(pg_current_wal_lsn(), flush_lsn)::float8 END AS flush_lag_bytes,
		CASE WHEN pg_is_in_recovery() THEN NULL
		     ELSE pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)::float8 END AS replay_lag_bytes,
		EXTRACT(EPOCH FROM write_lag)::float8 AS write_lag_seconds,
		EXTRACT(EPOCH FROM flush_lag)::float8 AS flush_lag_seconds,
		EXTRACT(EPOCH FROM replay_lag)::float8 AS replay_lag_seconds
	FROM pg_stat_replication`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
