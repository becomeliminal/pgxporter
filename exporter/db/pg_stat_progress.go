package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatProgressAnalyze returns one row per active ANALYZE (PG 13+).
func (db *Client) SelectPgStatProgressAnalyze(ctx context.Context) ([]*model.PgStatProgressAnalyze, error) {
	if !db.AtLeast(13, 0) {
		return nil, nil
	}
	rows := []*model.PgStatProgressAnalyze{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(datname, '') AS datname,
		relid::text AS relid,
		COALESCE(phase, '') AS phase,
		sample_blks_total,
		sample_blks_scanned,
		ext_stats_total,
		ext_stats_computed,
		child_tables_total,
		child_tables_done
	FROM pg_stat_progress_analyze`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}

// SelectPgStatProgressBasebackup returns one row per active pg_basebackup (PG 13+).
func (db *Client) SelectPgStatProgressBasebackup(ctx context.Context) ([]*model.PgStatProgressBasebackup, error) {
	if !db.AtLeast(13, 0) {
		return nil, nil
	}
	rows := []*model.PgStatProgressBasebackup{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(phase, '') AS phase,
		backup_total,
		backup_streamed,
		tablespaces_total,
		tablespaces_streamed
	FROM pg_stat_progress_basebackup`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}

// SelectPgStatProgressCopy returns one row per active COPY (PG 14+).
func (db *Client) SelectPgStatProgressCopy(ctx context.Context) ([]*model.PgStatProgressCopy, error) {
	if !db.AtLeast(14, 0) {
		return nil, nil
	}
	rows := []*model.PgStatProgressCopy{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(datname, '') AS datname,
		relid::text AS relid,
		COALESCE(command, '') AS command,
		COALESCE(type, '') AS type,
		bytes_total,
		bytes_processed,
		tuples_processed,
		tuples_excluded
	FROM pg_stat_progress_copy`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}

// SelectPgStatProgressCreateIndex returns one row per active CREATE INDEX / REINDEX.
func (db *Client) SelectPgStatProgressCreateIndex(ctx context.Context) ([]*model.PgStatProgressCreateIndex, error) {
	rows := []*model.PgStatProgressCreateIndex{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(datname, '') AS datname,
		relid::text AS relid,
		index_relid::text AS index_relid,
		COALESCE(command, '') AS command,
		COALESCE(phase, '') AS phase,
		lockers_total,
		lockers_done,
		blocks_total,
		blocks_done,
		tuples_total,
		tuples_done,
		partitions_total,
		partitions_done
	FROM pg_stat_progress_create_index`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}

// SelectPgStatProgressCluster returns one row per active CLUSTER / VACUUM FULL.
func (db *Client) SelectPgStatProgressCluster(ctx context.Context) ([]*model.PgStatProgressCluster, error) {
	rows := []*model.PgStatProgressCluster{}
	const sql = `SELECT
		current_database() AS database,
		COALESCE(datname, '') AS datname,
		relid::text AS relid,
		COALESCE(command, '') AS command,
		COALESCE(phase, '') AS phase,
		heap_tuples_scanned,
		heap_tuples_written,
		heap_blks_total,
		heap_blks_scanned,
		index_rebuild_count
	FROM pg_stat_progress_cluster`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
