package db

import (
	"strings"
	"testing"
)

func TestSqlSelectPgStatBgwriter_VersionGating(t *testing.T) {
	tests := []struct {
		name       string
		versionNum int
		wantCols   []string
		wantAbsent []string
	}{
		{
			name:       "PG 16 → full pre-split view",
			versionNum: 160010,
			wantCols:   []string{"buffers_clean", "checkpoints_timed", "checkpoint_write_time", "buffers_backend"},
		},
		{
			name:       "PG 17 → only the retained columns",
			versionNum: 170006,
			wantCols:   []string{"buffers_clean", "maxwritten_clean", "buffers_alloc", "stats_reset"},
			wantAbsent: []string{"checkpoints_timed", "checkpoint_write_time", "buffers_backend", "buffers_checkpoint"},
		},
		{
			name:       "PG 18 → same as PG 17",
			versionNum: 180000,
			wantAbsent: []string{"checkpoints_timed", "buffers_backend"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{ServerVersionNum: tc.versionNum}
			sql := c.sqlSelectPgStatBgwriter()
			for _, col := range tc.wantCols {
				if !strings.Contains(sql, col) {
					t.Errorf("PG %d SQL missing expected column %q: %s", tc.versionNum, col, sql)
				}
			}
			for _, col := range tc.wantAbsent {
				if strings.Contains(sql, ", "+col+",") ||
					strings.Contains(sql, ", "+col+" FROM") {
					t.Errorf("PG %d SQL unexpectedly contains column %q: %s", tc.versionNum, col, sql)
				}
			}
		})
	}
}
