package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatArchiver selects WAL-archiving stats. Available on all PG
// versions — no gating. Returns a single row (cluster-wide view).
func (db *Client) SelectPgStatArchiver(ctx context.Context) ([]*model.PgStatArchiver, error) {
	rows := []*model.PgStatArchiver{}
	const sql = `SELECT
		current_database() as database,
		archived_count,
		last_archived_time,
		failed_count,
		last_failed_time,
		stats_reset
	FROM pg_stat_archiver`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
