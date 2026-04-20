package db

import (
	"context"
	"strings"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatSubscription returns subscriber-side logical-replication
// state (PG 10+). Zero rows is the norm on any instance that isn't a
// logical-replication subscriber.
//
// PG 17 added worker_type; older servers don't have the column so we
// compose the projection per version.
func (db *Client) SelectPgStatSubscription(ctx context.Context) ([]*model.PgStatSubscription, error) {
	rows := []*model.PgStatSubscription{}
	if err := db.Select(ctx, &rows, db.sqlSelectPgStatSubscription()); err != nil {
		return nil, err
	}
	return rows, nil
}

func (db *Client) sqlSelectPgStatSubscription() string {
	cols := []string{
		"current_database() AS database",
		"subname",
		"pg_wal_lsn_diff(received_lsn, '0/0')::float8 AS received_lsn_bytes",
		"last_msg_send_time",
		"last_msg_receipt_time",
		"pg_wal_lsn_diff(latest_end_lsn, '0/0')::float8 AS latest_end_lsn_bytes",
		"latest_end_time",
	}
	if db.AtLeast(17, 0) {
		cols = append(cols, "COALESCE(worker_type, '') AS worker_type")
	} else {
		cols = append(cols, "'' AS worker_type")
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM pg_stat_subscription"
}
