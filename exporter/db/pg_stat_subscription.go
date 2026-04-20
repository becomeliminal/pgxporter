package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatSubscription aggregates pg_stat_subscription rows per
// subname. pid is intentionally excluded from the grouping because a
// single subscription can have multiple parallel apply/sync workers and
// pid values flap on worker restarts — summarising by subname gives a
// stable view of replication health.
//
// Available on all supported PG versions (10+; we require 13+).
func (db *Client) SelectPgStatSubscription(ctx context.Context) ([]*model.PgStatSubscription, error) {
	rows := []*model.PgStatSubscription{}
	const sql = `SELECT
		current_database() AS database,
		subname,
		bool_or(pid IS NOT NULL) AS active,
		(max(received_lsn) - '0/0')::int8 AS received_lsn_bytes,
		max(last_msg_send_time) AS last_msg_send_time,
		max(last_msg_receipt_time) AS last_msg_receipt_time,
		(max(latest_end_lsn) - '0/0')::int8 AS latest_end_lsn_bytes,
		max(latest_end_time) AS latest_end_time
	FROM pg_stat_subscription
	GROUP BY subname`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}

// SelectPgStatSubscriptionStats reads error counters from
// pg_stat_subscription_stats. The view was added in PG 15 — earlier
// versions return an empty slice without hitting the database.
func (db *Client) SelectPgStatSubscriptionStats(ctx context.Context) ([]*model.PgStatSubscriptionStats, error) {
	if !db.AtLeast(15, 0) {
		return nil, nil
	}
	rows := []*model.PgStatSubscriptionStats{}
	const sql = `SELECT
		current_database() AS database,
		subname,
		apply_error_count,
		sync_error_count,
		stats_reset
	FROM pg_stat_subscription_stats`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
