package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatSSL aggregates pg_stat_ssl into (ssl, version, cipher, bits)
// buckets with a connection count. Per-pid rows are deliberately discarded
// — they're per-backend and too noisy. Available on all supported PG
// versions.
func (db *Client) SelectPgStatSSL(ctx context.Context) ([]*model.PgStatSSL, error) {
	rows := []*model.PgStatSSL{}
	const sql = `SELECT
		current_database() AS database,
		ssl,
		coalesce(version, '') AS version,
		coalesce(cipher, '') AS cipher,
		coalesce(bits, 0) AS bits,
		count(*) AS connections
	FROM pg_stat_ssl
	GROUP BY ssl, version, cipher, bits`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
