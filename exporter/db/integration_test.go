//go:build integration

// Integration tests for the db package. Spin up a real Postgres via
// testutil.StartPG and run the SelectPg* queries against it, verifying
// the SQL composes correctly across PG 13–18 and version-gated columns
// behave as expected.
//
// Run:  go test -tags integration ./exporter/db/...
//
// Each sub-test is named after the PG version so you can target one:
//
//	go test -tags integration -run TestMatrix/PG_17_6 ./exporter/db/...
package db

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
	"github.com/becomeliminal/pgxporter/testutil"
)

// pgVersionsUnderTest is the matrix. Keep in sync with testutil.SupportedVersions.
var pgVersionsUnderTest = []struct {
	name    string
	version string
}{
	{"PG_13_22", "13.22"},
	{"PG_14_19", "14.19"},
	{"PG_15_14", "15.14"},
	{"PG_16_10", "16.10"},
	{"PG_17_6", "17.6"},
	{"PG_18_0", "18.0"},
}

// TestMatrix_VersionDetection verifies Client.probeServerVersion + AtLeast
// agree with the actual server version across the whole matrix.
func TestMatrix_VersionDetection(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)

			if client.ServerVersionNum == 0 {
				t.Fatal("probeServerVersion did not populate ServerVersionNum")
			}

			// e.g. 17.6 → major=17, which should match tc.version's first segment.
			wantMajor := parseMajor(t, tc.version)
			if got := client.ServerVersionNum / 10000; got != wantMajor {
				t.Errorf("ServerVersionNum major = %d, want %d (server_version_num=%d)",
					got, wantMajor, client.ServerVersionNum)
			}

			// AtLeast(major, 0) should be true.
			if !client.AtLeast(wantMajor, 0) {
				t.Errorf("AtLeast(%d, 0) = false, want true", wantMajor)
			}
			// AtLeast(major+1, 0) should be false.
			if client.AtLeast(wantMajor+1, 0) {
				t.Errorf("AtLeast(%d, 0) = true, want false", wantMajor+1)
			}
		})
	}
}

// TestMatrix_SelectPgStatDatabase verifies the version-gated SELECT executes
// cleanly across the matrix and returns at least one non-template DB row.
func TestMatrix_SelectPgStatDatabase(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatDatabase(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatDatabase: %v", err)
			}
			if len(rows) == 0 {
				t.Fatalf("expected at least one pg_stat_database row, got 0")
			}

			// PG 14+ should populate Sessions for the postgres DB once anyone
			// has connected (which we did via connectPG).
			if client.AtLeast(14, 0) {
				foundValid := false
				for _, r := range rows {
					if r.Sessions.Valid {
						foundValid = true
						break
					}
				}
				if !foundValid {
					t.Errorf("PG %s: expected at least one row with Sessions.Valid=true", tc.version)
				}
			}
		})
	}
}

// TestMatrix_SelectPgStatBgwriter verifies the version-gated SELECT executes
// on every PG version. PG <17 should return the pre-split columns; PG 17+
// should only return the retained columns.
func TestMatrix_SelectPgStatBgwriter(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatBgwriter(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatBgwriter: %v", err)
			}
			if len(rows) == 0 {
				t.Fatalf("expected one pg_stat_bgwriter row, got 0")
			}

			r := rows[0]
			if !r.BuffersAlloc.Valid {
				t.Errorf("PG %s: BuffersAlloc should be Valid (always exposed)", tc.version)
			}
			if client.AtLeast(17, 0) {
				if r.CheckpointsTimed.Valid {
					t.Errorf("PG %s: CheckpointsTimed should NOT be Valid (moved to pg_stat_checkpointer)", tc.version)
				}
			} else {
				if !r.CheckpointsTimed.Valid {
					t.Errorf("PG %s: CheckpointsTimed should be Valid on pre-17", tc.version)
				}
			}
		})
	}
}

// TestMatrix_SelectPgStatCheckpointer verifies the PG 17+ view returns rows
// on 17/18 and cleanly returns nothing on older servers.
func TestMatrix_SelectPgStatCheckpointer(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatCheckpointer(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatCheckpointer: %v", err)
			}
			if client.AtLeast(17, 0) {
				if len(rows) == 0 {
					t.Fatalf("PG %s: expected 1 pg_stat_checkpointer row, got 0", tc.version)
				}
			} else {
				if len(rows) != 0 {
					t.Fatalf("PG %s: expected 0 rows (view does not exist pre-17), got %d", tc.version, len(rows))
				}
			}
		})
	}
}

// TestMatrix_SelectPgStatArchiver verifies the SELECT executes cleanly on
// every supported PG version and returns the single cluster-wide row.
func TestMatrix_SelectPgStatArchiver(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatArchiver(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatArchiver: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("PG %s: expected exactly 1 pg_stat_archiver row, got %d", tc.version, len(rows))
			}
			// Counters are always non-NULL; last_*_time may be NULL on a fresh
			// server with no archive_command configured. Only assert the shape.
			if !rows[0].ArchivedCount.Valid || !rows[0].FailedCount.Valid {
				t.Errorf("PG %s: archived_count / failed_count should be Valid", tc.version)
			}
		})
	}
}

// TestMatrix_SelectPgStatReplication verifies the SELECT executes cleanly.
// A fresh PG has no replicas attached so we expect zero rows; the test is a
// regression guard for column renames / LSN-function availability.
func TestMatrix_SelectPgStatReplication(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatReplication(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatReplication: %v", err)
			}
			// Fresh PG has no attached replicas.
			if len(rows) != 0 {
				t.Logf("PG %s: got %d replication rows (unexpected but not an error)", tc.version, len(rows))
			}
		})
	}
}

// TestMatrix_SelectPgStatWalReceiver verifies the SELECT executes cleanly.
// A fresh PG (primary) has no wal_receiver attached so we expect zero rows;
// the test is a regression guard for column renames across versions.
func TestMatrix_SelectPgStatWalReceiver(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatWalReceiver(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatWalReceiver: %v", err)
			}
			// Fresh PG is a primary — view is empty.
			if len(rows) != 0 {
				t.Logf("PG %s: got %d wal_receiver rows (unexpected on a primary but not an error)", tc.version, len(rows))
			}
		})
	}
}

// TestMatrix_SelectPgReplicationSlots verifies the version-gated SELECT
// executes cleanly across the matrix. Fresh PG has no slots so zero rows is
// expected; the test is a schema-drift guard for wal_status / safe_wal_size
// (PG 13+) and conflicting (PG 16+).
func TestMatrix_SelectPgReplicationSlots(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgReplicationSlots(ctx)
			if err != nil {
				t.Fatalf("SelectPgReplicationSlots: %v", err)
			}
			// No slots on a fresh PG.
			_ = rows
		})
	}
}

// TestMatrix_SelectPgDatabaseSize verifies pg_database_size() executes and
// returns at least one row for the built-in postgres database.
func TestMatrix_SelectPgDatabaseSize(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgDatabaseSize(ctx)
			if err != nil {
				t.Fatalf("SelectPgDatabaseSize: %v", err)
			}
			if len(rows) == 0 {
				t.Fatalf("PG %s: expected at least one pg_database_size row, got 0", tc.version)
			}
			for _, r := range rows {
				if !r.Bytes.Valid || r.Bytes.Int64 <= 0 {
					t.Errorf("PG %s: datname=%q has non-positive bytes %d", tc.version, r.DatName.String, r.Bytes.Int64)
				}
			}
		})
	}
}

// TestMatrix_SelectPgStatWal verifies the PG 14+ SELECT executes cleanly on
// PG 14+ and returns nothing on pre-14. Tests the column-composition split
// between PG 14 (wal_write/wal_sync present) and PG 15+ (those removed).
func TestMatrix_SelectPgStatWal(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatWal(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatWal: %v", err)
			}
			if client.AtLeast(14, 0) {
				if len(rows) == 0 {
					t.Fatalf("PG %s: expected 1 pg_stat_wal row, got 0", tc.version)
				}
				r := rows[0]
				if !r.WalRecords.Valid {
					t.Errorf("PG %s: wal_records should be Valid", tc.version)
				}
				// Wal_write is gated to PG 14 only.
				wantWriteValid := !client.AtLeast(15, 0)
				if r.WalWrite.Valid != wantWriteValid {
					t.Errorf("PG %s: wal_write.Valid = %v, want %v", tc.version, r.WalWrite.Valid, wantWriteValid)
				}
			} else {
				if len(rows) != 0 {
					t.Fatalf("PG %s: expected 0 rows on pre-14, got %d", tc.version, len(rows))
				}
			}
		})
	}
}

// TestMatrix_SelectPgStatActivity verifies the aggregate SELECT returns at
// least one row (the exporter's own connection) on every PG version and
// that the bucket fields (state, wait_event_type, backend_type) are all
// populated.
func TestMatrix_SelectPgStatActivity(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatActivity(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatActivity: %v", err)
			}
			if len(rows) == 0 {
				t.Fatalf("PG %s: expected at least one pg_stat_activity bucket, got 0", tc.version)
			}
			// Sanity-check that our own backend shows up with backend_type populated.
			for _, r := range rows {
				if !r.State.Valid || !r.WaitEventType.Valid || !r.BackendType.Valid {
					t.Errorf("PG %s: bucket missing label field (state/wait_event_type/backend_type)", tc.version)
				}
				if !r.Count.Valid || r.Count.Int64 <= 0 {
					t.Errorf("PG %s: bucket has non-positive count %d", tc.version, r.Count.Int64)
				}
			}
		})
	}
}

// TestMatrix_SelectPgStatUserTables runs the version-gated SELECT against
// a live server and asserts it completes without error. Regression guard
// for column renames / schema drift when PG releases a new major.
func TestMatrix_SelectPgStatUserTables(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rows, err := client.SelectPgStatUserTables(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatUserTables: %v (SQL: %s)", err, client.sqlSelectPgStatUserTables())
			}
			// A fresh PG has no user tables — SELECT returns zero rows, which is fine.
			_ = rows
		})
	}
}

// TestMatrix_SelectPgStatUserTables_WithFixtures exercises the scan path
// with real rows, including the PG 17+ partitioned-table NULL shape that
// originally triggered LIM-925 ("cannot assign 0 1 into *int").
func TestMatrix_SelectPgStatUserTables_WithFixtures(t *testing.T) {
	for _, tc := range pgVersionsUnderTest {
		t.Run(tc.name, func(t *testing.T) {
			client := connectPG(t, tc.version)

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Create a regular table + a partitioned parent (PG 17+).
			setup := []string{
				`CREATE TABLE simple (id int)`,
				`INSERT INTO simple VALUES (1), (2), (3)`,
			}
			if client.AtLeast(17, 0) {
				setup = append(setup,
					`CREATE TABLE partitioned_parent (id int) PARTITION BY RANGE (id)`,
					`CREATE TABLE partitioned_child PARTITION OF partitioned_parent FOR VALUES FROM (0) TO (100)`,
					`INSERT INTO partitioned_parent VALUES (1), (2), (3)`,
				)
			}
			for _, stmt := range setup {
				if _, err := client.pool.Exec(ctx, stmt); err != nil {
					t.Fatalf("%s: %v", stmt, err)
				}
			}

			// Force stats flush so pg_stat_user_tables reflects our inserts.
			if _, err := client.pool.Exec(ctx, `SELECT pg_stat_force_next_flush()`); err != nil {
				// Older PGs may not have that function — tolerate.
				t.Logf("pg_stat_force_next_flush unavailable on PG %s: %v", tc.version, err)
			}

			rows, err := client.SelectPgStatUserTables(ctx)
			if err != nil {
				t.Fatalf("SelectPgStatUserTables: %v", err)
			}
			if len(rows) == 0 {
				t.Fatalf("expected at least one user-table stat row, got 0")
			}

			// Spot-check: version-gated fields should be Valid on the right versions.
			if client.AtLeast(13, 0) {
				anyValid := false
				for _, r := range rows {
					if r.NInsSinceVacuum.Valid {
						anyValid = true
						break
					}
				}
				if !anyValid {
					t.Errorf("PG %s: expected at least one row with NInsSinceVacuum.Valid=true", tc.version)
				}
			}

			if client.AtLeast(17, 0) {
				// Partitioned parent row — some counters should be NULL.
				var parentRow *model.PgStatUserTable
				for _, r := range rows {
					if r.RelName.String == "partitioned_parent" {
						parentRow = r
						break
					}
				}
				if parentRow == nil {
					t.Fatalf("PG %s: partitioned_parent row missing from pg_stat_user_tables", tc.version)
				}
				// The partitioned parent itself has no heap → stats for some columns
				// come through as NULL. That's the shape that triggered LIM-925.
				// We don't assert any specific column is NULL (PG has flexibility);
				// we just verify the scan completed without error, which is what
				// matters for the regression.
			}
		})
	}
}

// connectPG starts a PG and returns a connected Client. Cleanup is
// registered via t.Cleanup so tests don't need explicit teardown.
func connectPG(t *testing.T, version string) *Client {
	t.Helper()
	pg := testutil.StartPG(t, version)

	opts := Opts{
		Host:                  pg.Host,
		Port:                  pg.Port,
		User:                  "postgres",
		Database:              "postgres",
		ApplicationName:       "pgxporter-integration-test",
		MaxConnectionRetries:  0,
		ConnectTimeout:        10 * time.Second,
		PoolMaxConns:          2,
		PoolMinConns:          1,
		PoolHealthCheckPeriod: time.Minute,
		PoolMaxConnLifetime:   time.Hour,
		PoolMaxConnIdleTime:   30 * time.Minute,
		StatementCacheMode:    "prepare",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := New(ctx, opts)
	if err != nil {
		t.Fatalf("db.New against PG %s: %v", version, err)
	}
	t.Cleanup(func() { client.pool.Close() })
	return client
}

// parseMajor pulls the major number out of "17.6" / "13.22".
func parseMajor(t *testing.T, version string) int {
	t.Helper()
	for i, c := range version {
		if c == '.' {
			maj, err := strconv.Atoi(version[:i])
			if err != nil {
				t.Fatalf("parseMajor(%q): %v", version, err)
			}
			return maj
		}
	}
	maj, err := strconv.Atoi(version)
	if err != nil {
		t.Fatalf("parseMajor(%q): %v", version, err)
	}
	return maj
}
