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

// TestStartPostgresExporter is a smoke test: download the binary (or reuse
// cache), boot it pointed at a fresh PG, hit /metrics, confirm the response
// contains the expected postgres_exporter self-metric.
func TestStartPostgresExporter(t *testing.T) {
	pg := testutil.StartPG(t, "17.6")
	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=postgres sslmode=disable", pg.Port)

	pe := testutil.StartPostgresExporter(t, dsn)
	t.Logf("postgres_exporter listening at %s", pe.URL)

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

	// Self-metric postgres_exporter always exposes. Confirms we actually
	// scraped it rather than a 200-OK-on-empty-handler.
	if !strings.Contains(string(body), "pg_exporter_") && !strings.Contains(string(body), "pg_up") {
		t.Errorf("/metrics response lacks expected postgres_exporter self-metric; first 500 bytes: %s",
			string(body[:min(500, len(body))]))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
