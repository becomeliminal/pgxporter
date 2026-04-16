package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

const sqlSelectPgStatIOUserIndexes = `
SELECT
    current_database() as database,
    schemaname,
    relname,
    indexrelname,
    idx_blks_read,
    idx_blks_hit
FROM pg_statio_user_indexes`

// SelectPgStatIOUserIndexes selects stats on user indexes.
func (db *Client) SelectPgStatIOUserIndexes(ctx context.Context) ([]*model.PgStatIOUserIndex, error) {
	pgStatIOUserIndexes := []*model.PgStatIOUserIndex{}
	if err := db.Select(ctx, &pgStatIOUserIndexes, sqlSelectPgStatIOUserIndexes); err != nil {
		return nil, err
	}
	return pgStatIOUserIndexes, nil
}
