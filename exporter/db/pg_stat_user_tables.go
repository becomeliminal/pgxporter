package db

import (
	"context"
	"strings"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatUserTables selects stats on user tables. Version-gated columns
// (see [model.PgStatUserTable]) are only projected if the connected server
// is new enough; on older servers the corresponding struct fields remain
// zero-valued and collectors skip them via their .Valid check.
func (db *Client) SelectPgStatUserTables(ctx context.Context) ([]*model.PgStatUserTable, error) {
	pgStatUserTables := []*model.PgStatUserTable{}
	if err := db.Select(ctx, &pgStatUserTables, db.sqlSelectPgStatUserTables()); err != nil {
		return nil, err
	}
	return pgStatUserTables, nil
}

// sqlSelectPgStatUserTables composes the SELECT based on the cached server
// version. Missing columns simply aren't projected — pgtype fields stay
// invalid and emitting is skipped at the collector layer.
func (db *Client) sqlSelectPgStatUserTables() string {
	cols := []string{
		"current_database() as database",
		"schemaname",
		"relname",
		"seq_scan",
		"seq_tup_read",
		"idx_scan",
		"idx_tup_fetch",
		"n_tup_ins",
		"n_tup_upd",
		"n_tup_del",
		"n_tup_hot_upd",
		"n_live_tup",
		"n_dead_tup",
		"n_mod_since_analyze",
		"last_vacuum",
		"last_autovacuum",
		"last_analyze",
		"last_autoanalyze",
		"vacuum_count",
		"autovacuum_count",
		"analyze_count",
		"autoanalyze_count",
	}
	if db.AtLeast(13, 0) {
		cols = append(cols, "n_ins_since_vacuum")
	}
	if db.AtLeast(16, 0) {
		cols = append(cols, "last_seq_scan", "last_idx_scan")
	}
	if db.AtLeast(17, 0) {
		cols = append(cols, "n_tup_newpage_upd")
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM pg_stat_user_tables"
}
