package db

import (
	"strings"
	"testing"
)

func TestSqlSelectPgStatDatabase_VersionGating(t *testing.T) {
	tests := []struct {
		name       string
		versionNum int
		wantCols   []string
		wantAbsent []string
	}{
		{
			name:       "PG 11 (pre-checksum + pre-session-time)",
			versionNum: 110018,
			wantCols:   []string{"xact_commit", "blks_read", "deadlocks"},
			wantAbsent: []string{"checksum_failures", "session_time", "sessions_abandoned"},
		},
		{
			name:       "PG 12 adds checksum_failures",
			versionNum: 120012,
			wantCols:   []string{"checksum_failures", "checksum_last_failure"},
			wantAbsent: []string{"session_time", "sessions_abandoned"},
		},
		{
			name:       "PG 13 keeps checksum_failures but not session_time",
			versionNum: 130022,
			wantCols:   []string{"checksum_failures"},
			wantAbsent: []string{"session_time", "sessions_abandoned"},
		},
		{
			name:       "PG 14 adds session_time + sessions_*",
			versionNum: 140019,
			wantCols:   []string{"checksum_failures", "session_time", "active_time", "sessions_abandoned"},
		},
		{
			name:       "PG 17 keeps all",
			versionNum: 170006,
			wantCols:   []string{"checksum_failures", "session_time", "sessions_killed"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{ServerVersionNum: tc.versionNum}
			sql := c.sqlSelectPgStatDatabase()
			for _, col := range tc.wantCols {
				if !strings.Contains(sql, col) {
					t.Errorf("PG %d SQL missing expected column %q", tc.versionNum, col)
				}
			}
			for _, col := range tc.wantAbsent {
				if strings.Contains(sql, ", "+col+",") ||
					strings.Contains(sql, ", "+col+" FROM") ||
					strings.Contains(sql, ", "+col+" WHERE") {
					t.Errorf("PG %d SQL unexpectedly contains column %q: %s", tc.versionNum, col, sql)
				}
			}
		})
	}
}
