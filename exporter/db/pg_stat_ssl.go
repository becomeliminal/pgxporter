package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatSSL aggregates pg_stat_ssl by (ssl, version, cipher, bits)
// to keep cardinality bounded. Backends that terminate TLS at the
// postmaster show ssl=true; non-TLS (e.g. Unix-socket) clients show
// ssl=false with empty version/cipher.
func (db *Client) SelectPgStatSSL(ctx context.Context) ([]*model.PgStatSSL, error) {
	rows := []*model.PgStatSSL{}
	const sql = `SELECT
		current_database() AS database,
		ssl,
		COALESCE(version, '') AS version,
		COALESCE(cipher, '') AS cipher,
		COALESCE(bits, 0)::bigint AS bits,
		count(*) AS count
	FROM pg_stat_ssl
	GROUP BY ssl, version, cipher, bits`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
