package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

const sqlSelectPgStatIOUserTables = `
SELECT
    current_database() as database,
    schemaname,
    relname,
    heap_blks_read,
    heap_blks_hit,
    idx_blks_read,
    idx_blks_hit,
    toast_blks_read,
    toast_blks_hit,
    tidx_blks_read,
    tidx_blks_hit
FROM pg_statio_user_tables`

// SelectPgStatIOUserTables selects stats on user tables.
func (db *Client) SelectPgStatIOUserTables(ctx context.Context) ([]*model.PgStatIOUserTable, error) {
	pgStatIOUserTables := []*model.PgStatIOUserTable{}
	if err := db.Select(ctx, &pgStatIOUserTables, sqlSelectPgStatIOUserTables); err != nil {
		return nil, err
	}
	return pgStatIOUserTables, nil
}
