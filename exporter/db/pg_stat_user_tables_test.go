package db

import (
	"strings"
	"testing"
)

// TestSqlSelectPgStatUserTables_VersionGating verifies the composed SELECT
// includes version-gated columns iff the cached server version is new enough.
func TestSqlSelectPgStatUserTables_VersionGating(t *testing.T) {
	tests := []struct {
		name       string
		versionNum int
		wantCols   []string
		wantAbsent []string
	}{
		{
			name:       "zero version → base columns only",
			versionNum: 0,
			wantAbsent: []string{"n_ins_since_vacuum", "last_seq_scan", "last_idx_scan", "n_tup_newpage_upd"},
		},
		{
			name:       "PG 12 → no new columns",
			versionNum: 120012,
			wantCols:   []string{"seq_scan", "n_tup_ins", "autoanalyze_count"},
			wantAbsent: []string{"n_ins_since_vacuum", "last_seq_scan", "last_idx_scan", "n_tup_newpage_upd"},
		},
		{
			name:       "PG 13 adds n_ins_since_vacuum",
			versionNum: 130022,
			wantCols:   []string{"seq_scan", "n_ins_since_vacuum"},
			wantAbsent: []string{"last_seq_scan", "last_idx_scan", "n_tup_newpage_upd"},
		},
		{
			name:       "PG 15 still no last_seq_scan",
			versionNum: 150014,
			wantCols:   []string{"n_ins_since_vacuum"},
			wantAbsent: []string{"last_seq_scan", "last_idx_scan", "n_tup_newpage_upd"},
		},
		{
			name:       "PG 16 adds last_seq_scan + last_idx_scan",
			versionNum: 160010,
			wantCols:   []string{"n_ins_since_vacuum", "last_seq_scan", "last_idx_scan"},
			wantAbsent: []string{"n_tup_newpage_upd"},
		},
		{
			name:       "PG 17 adds n_tup_newpage_upd",
			versionNum: 170006,
			wantCols:   []string{"n_ins_since_vacuum", "last_seq_scan", "last_idx_scan", "n_tup_newpage_upd"},
		},
		{
			name:       "PG 18 keeps all",
			versionNum: 180000,
			wantCols:   []string{"n_ins_since_vacuum", "last_seq_scan", "last_idx_scan", "n_tup_newpage_upd"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{ServerVersionNum: tc.versionNum}
			sql := c.sqlSelectPgStatUserTables()
			for _, col := range tc.wantCols {
				if !strings.Contains(sql, col) {
					t.Errorf("PG %d SQL missing expected column %q: %s", tc.versionNum, col, sql)
				}
			}
			for _, col := range tc.wantAbsent {
				// Use word-boundary-ish check: suffix "," or " FROM" so "idx_scan"
				// doesn't false-positive on "last_idx_scan".
				if strings.Contains(sql, ", "+col+",") ||
					strings.Contains(sql, ", "+col+" FROM") ||
					strings.HasSuffix(strings.TrimSpace(sql), col) {
					t.Errorf("PG %d SQL unexpectedly contains column %q: %s", tc.versionNum, col, sql)
				}
			}
		})
	}
}
