package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatCheckpointer selects checkpoint stats. PG 17 introduced this
// view (splitting columns out of pg_stat_bgwriter); on pre-17 servers it
// doesn't exist and the method returns an empty result without error, so
// the collector emits no metrics.
func (db *Client) SelectPgStatCheckpointer(ctx context.Context) ([]*model.PgStatCheckpointer, error) {
	if !db.AtLeast(17, 0) {
		return nil, nil
	}
	rows := []*model.PgStatCheckpointer{}
	const sql = `SELECT
		current_database() as database,
		num_timed,
		num_requested,
		restartpoints_timed,
		restartpoints_req,
		restartpoints_done,
		write_time,
		sync_time,
		buffers_written,
		stats_reset
	FROM pg_stat_checkpointer`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
