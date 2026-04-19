package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgDatabaseSize returns pg_database_size() for every non-template
// database. Template databases (template0/template1) are excluded to match
// what postgres_exporter reports.
func (db *Client) SelectPgDatabaseSize(ctx context.Context) ([]*model.PgDatabaseSize, error) {
	rows := []*model.PgDatabaseSize{}
	const sql = `SELECT
		current_database() AS database,
		datname,
		pg_database_size(datname) AS bytes
	FROM pg_database
	WHERE datallowconn AND NOT datistemplate`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
