//go:build integration

package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestStartPG is the smoke test: download → initdb → start → connect → SELECT 1.
// Guarded by -tags integration because it downloads ~10MB on first run.
func TestStartPG(t *testing.T) {
	pg := StartPG(t, "17.6")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, pg.DSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	var n int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&n); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if n != 1 {
		t.Fatalf("SELECT 1 returned %d, want 1", n)
	}

	var ver int
	if err := conn.QueryRow(ctx, "SHOW server_version_num").Scan(&ver); err != nil {
		// pgx decodes SHOW into int if configured; if not, accept string path.
		var vs string
		if err2 := conn.QueryRow(ctx, "SELECT current_setting('server_version_num')").Scan(&vs); err2 != nil {
			t.Fatalf("server_version_num: %v / %v", err, err2)
		}
		t.Logf("server_version_num=%s", vs)
	} else {
		t.Logf("server_version_num=%d", ver)
		if ver/10000 != 17 {
			t.Errorf("expected PG 17, got major=%d", ver/10000)
		}
	}
}

// TestCacheHit confirms the harness caches binaries between calls.
// Second call should be instant (no network).
func TestCacheHit(t *testing.T) {
	// Warm cache.
	pg1 := StartPG(t, "17.6")
	pg1.Stop()

	// Time the second start; should be < 5s without network.
	start := time.Now()
	pg2 := StartPG(t, "17.6")
	defer pg2.Stop()
	elapsed := time.Since(start)
	t.Logf("second StartPG took %v", elapsed)
	if elapsed > 10*time.Second {
		t.Errorf("second start took %v, expected fast cache hit", elapsed)
	}
}
