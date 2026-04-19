package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatProgressVacuum returns one row per active VACUUM worker.
// Zero rows is the steady state on a healthy cluster.
func (db *Client) SelectPgStatProgressVacuum(ctx context.Context) ([]*model.PgStatProgressVacuum, error) {
	rows := []*model.PgStatProgressVacuum{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(datname, '') AS datname,
		relid::text AS relid,
		COALESCE(phase, '') AS phase,
		heap_blks_total,
		heap_blks_scanned,
		heap_blks_vacuumed,
		index_vacuum_count
	FROM pg_stat_progress_vacuum`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
