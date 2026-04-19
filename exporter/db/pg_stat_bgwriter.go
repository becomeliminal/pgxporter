package db

import (
	"context"
	"strings"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgStatBgwriter selects background-writer statistics.
//
// PG 17 split this view — the checkpoint + backend-buffer columns moved to
// pg_stat_checkpointer (see SelectPgStatCheckpointer). SelectPgStatBgwriter
// composes the SELECT based on the server version, leaving the migrated
// fields NULL (Valid=false) on PG 17+.
func (db *Client) SelectPgStatBgwriter(ctx context.Context) ([]*model.PgStatBgwriter, error) {
	rows := []*model.PgStatBgwriter{}
	if err := db.Select(ctx, &rows, db.sqlSelectPgStatBgwriter()); err != nil {
		return nil, err
	}
	return rows, nil
}

func (db *Client) sqlSelectPgStatBgwriter() string {
	cols := []string{
		"current_database() as database",
		"buffers_clean",
		"maxwritten_clean",
		"buffers_alloc",
		"stats_reset",
	}
	if !db.AtLeast(17, 0) {
		// PG < 17 exposes the checkpoint + backend-buffer columns inline.
		cols = append(cols,
			"checkpoints_timed",
			"checkpoints_req",
			"checkpoint_write_time",
			"checkpoint_sync_time",
			"buffers_checkpoint",
			"buffers_backend",
			"buffers_backend_fsync",
		)
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM pg_stat_bgwriter"
}
