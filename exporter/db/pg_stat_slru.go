package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatSLRU returns SLRU buffer-pool stats (PG 13+). One row per
// named pool. Returns nil on pre-13 servers.
func (db *Client) SelectPgStatSLRU(ctx context.Context) ([]*model.PgStatSLRU, error) {
	if !db.AtLeast(13, 0) {
		return nil, nil
	}
	rows := []*model.PgStatSLRU{}
	const sql = `SELECT
		current_database() AS database,
		name,
		blks_zeroed,
		blks_hit,
		blks_read,
		blks_written,
		blks_exists,
		flushes,
		truncates,
		stats_reset
	FROM pg_stat_slru`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
