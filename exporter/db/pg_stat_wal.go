package db

import (
	"context"
	"strings"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatWal selects cluster-wide WAL stats. Returns nil on pre-14
// servers (view doesn't exist). PG 15+ dropped wal_write/wal_sync and
// their _time counterparts — the SQL composes per-version so those
// columns come through NULL on PG 15+.
func (db *Client) SelectPgStatWal(ctx context.Context) ([]*model.PgStatWal, error) {
	if !db.AtLeast(14, 0) {
		return nil, nil
	}
	rows := []*model.PgStatWal{}
	if err := db.Select(ctx, &rows, db.sqlSelectPgStatWal()); err != nil {
		return nil, err
	}
	return rows, nil
}

func (db *Client) sqlSelectPgStatWal() string {
	cols := []string{
		"current_database() AS database",
		"wal_records",
		"wal_fpi",
		"wal_bytes::float8 AS wal_bytes",
		"wal_buffers_full",
		"stats_reset",
	}
	// PG 14 only — removed in 15.
	if db.AtLeast(14, 0) && !db.AtLeast(15, 0) {
		cols = append(cols,
			"wal_write",
			"wal_sync",
			"wal_write_time",
			"wal_sync_time",
		)
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM pg_stat_wal"
}
