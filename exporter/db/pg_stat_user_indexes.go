package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

const sqlSelectPgStatUserIndexes = `
SELECT
    current_database() as database,
    schemaname,
    relname,
    indexrelname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes`

// SelectPgStatUserIndexes selects stats on user indexes.
func (db *Client) SelectPgStatUserIndexes(ctx context.Context) ([]*model.PgStatUserIndex, error) {
	pgStatUserIndexes := []*model.PgStatUserIndex{}
	if err := db.Select(ctx, &pgStatUserIndexes, sqlSelectPgStatUserIndexes); err != nil {
		return nil, err
	}
	return pgStatUserIndexes, nil
}
