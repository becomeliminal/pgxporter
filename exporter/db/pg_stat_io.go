package db

import (
	"context"
	"strings"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatIO returns cross-cutting I/O stats (PG 16+). Returns nil on
// older servers — the view doesn't exist there and the collector emits
// nothing. One row per (backend_type, object, context) bucket; many
// counter columns are NULL for buckets where the combination isn't
// meaningful (e.g. "writes" for a backend that only reads).
//
// PG 18 removed op_bytes (replaced with the per-op *_bytes columns added
// in PG 17); the SQL composes the projection per-version so the field
// stays NULL on PG 18+.
func (db *Client) SelectPgStatIO(ctx context.Context) ([]*model.PgStatIO, error) {
	if !db.AtLeast(16, 0) {
		return nil, nil
	}
	rows := []*model.PgStatIO{}
	if err := db.Select(ctx, &rows, db.sqlSelectPgStatIO()); err != nil {
		return nil, err
	}
	return rows, nil
}

func (db *Client) sqlSelectPgStatIO() string {
	cols := []string{
		"current_database() AS database",
		"COALESCE(backend_type, '') AS backend_type",
		"COALESCE(object, '') AS object",
		"COALESCE(context, '') AS context",
		"reads",
		"read_time",
		"writes",
		"write_time",
		"writebacks",
		"writeback_time",
		"extends",
		"extend_time",
		"hits",
		"evictions",
		"reuses",
		"fsyncs",
		"fsync_time",
		"stats_reset",
	}
	// op_bytes was removed in PG 18.
	if !db.AtLeast(18, 0) {
		cols = append(cols, "op_bytes")
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM pg_stat_io"
}
