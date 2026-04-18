package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

const sqlSelectPgStatUserTables = `
SELECT
     current_database() as database,
     schemaname,
     relname,
     seq_scan,
     seq_tup_read,
     idx_scan,
     idx_tup_fetch,
     n_tup_ins,
     n_tup_upd,
     n_tup_del,
     n_tup_hot_upd,
     n_live_tup,
     n_dead_tup,
     n_mod_since_analyze,
     last_vacuum,
     last_autovacuum,
     last_analyze,
     last_autoanalyze,
     vacuum_count,
     autovacuum_count,
     analyze_count,
     autoanalyze_count
FROM pg_stat_user_tables`

// SelectPgStatUserTables selects stats on user tables.
func (db *Client) SelectPgStatUserTables(ctx context.Context) ([]*model.PgStatUserTable, error) {
	pgStatUserTables := []*model.PgStatUserTable{}
	if err := db.Select(ctx, &pgStatUserTables, sqlSelectPgStatUserTables); err != nil {
		return nil, err
	}
	return pgStatUserTables, nil
}
