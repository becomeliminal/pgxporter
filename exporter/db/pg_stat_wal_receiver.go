package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatWalReceiver selects standby-side WAL-receiver stats. On a
// primary this view is empty and returns zero rows. LSN columns are
// converted to byte offsets via pg_wal_lsn_diff(lsn, '0/0').
func (db *Client) SelectPgStatWalReceiver(ctx context.Context) ([]*model.PgStatWalReceiver, error) {
	rows := []*model.PgStatWalReceiver{}
	const sql = `SELECT
		current_database() AS database,
		pid,
		status,
		pg_wal_lsn_diff(receive_start_lsn, '0/0')::float8 AS receive_start_lsn_bytes,
		pg_wal_lsn_diff(written_lsn, '0/0')::float8 AS written_lsn_bytes,
		pg_wal_lsn_diff(flushed_lsn, '0/0')::float8 AS flushed_lsn_bytes,
		pg_wal_lsn_diff(latest_end_lsn, '0/0')::float8 AS latest_end_lsn_bytes,
		last_msg_send_time,
		last_msg_receipt_time,
		latest_end_time,
		slot_name,
		sender_host
	FROM pg_stat_wal_receiver`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
