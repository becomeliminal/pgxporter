package db

import (
	"context"
	"strings"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgReplicationSlots selects per-slot replication state. Zero rows is
// the common case (no slots created yet). The column list is composed at
// runtime based on the PG version — wal_status / safe_wal_size arrived in
// PG 13 and conflicting in PG 16.
func (db *Client) SelectPgReplicationSlots(ctx context.Context) ([]*model.PgReplicationSlot, error) {
	rows := []*model.PgReplicationSlot{}
	if err := db.Select(ctx, &rows, db.sqlSelectPgReplicationSlots()); err != nil {
		return nil, err
	}
	return rows, nil
}

func (db *Client) sqlSelectPgReplicationSlots() string {
	cols := []string{
		"current_database() AS database",
		"slot_name",
		"COALESCE(plugin, '') AS plugin",
		"slot_type",
		"COALESCE(database, '') AS datname",
		"active",
		"temporary",
		"pg_wal_lsn_diff(restart_lsn, '0/0')::float8 AS restart_lsn_bytes",
		"pg_wal_lsn_diff(confirmed_flush_lsn, '0/0')::float8 AS confirmed_flush_lsn_bytes",
		"CASE WHEN pg_is_in_recovery() THEN NULL " +
			"ELSE pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)::float8 END AS retained_wal_bytes",
	}
	if db.AtLeast(13, 0) {
		cols = append(cols,
			"wal_status",
			"safe_wal_size AS safe_wal_size_bytes",
		)
	}
	if db.AtLeast(16, 0) {
		cols = append(cols, "conflicting")
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM pg_replication_slots"
}
