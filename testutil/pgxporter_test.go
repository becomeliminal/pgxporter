//go:build integration

package testutil_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/becomeliminal/pgxporter/testutil"
)

// TestStartPgxporter is a smoke test: compile the pgxporter CLI, boot it
// against a fresh PG, hit /metrics, confirm the response contains an
// expected pgxporter self-metric. This guards against regressions that
// break either the cmd/ binary's flag parsing / DSN handling or the
// testutil harness's compile-and-boot flow.
func TestStartPgxporter(t *testing.T) {
	pg := testutil.StartPG(t, "17.6")
	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=postgres sslmode=disable", pg.Port)

	pe := testutil.StartPgxporter(t, dsn)
	t.Logf("pgxporter listening at %s", pe.URL)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(pe.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics: status %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("empty /metrics response")
	}

	// pgxporter's self-metric — present on every scrape regardless of PG state.
	if !strings.Contains(string(body), "pg_stat_up") {
		preview := body
		if len(preview) > 500 {
			preview = preview[:500]
		}
		t.Errorf("/metrics response lacks pg_stat_up; first 500 bytes: %s", preview)
	}
}
