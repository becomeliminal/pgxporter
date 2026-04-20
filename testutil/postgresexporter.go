package testutil

// Downloader + harness for prometheus-community/postgres_exporter. Used by
// the head-to-head benchmark under exporter/headtohead_bench_test.go so
// reviewers can reproduce the numbers without a manual install.
//
// Same structural pattern as StartPG: cache tarballs under
// ~/.cache/pgxporter/postgres-exporter, boot on an ephemeral port, register
// t.Cleanup to kill the process when the test ends.

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// PostgresExporterReleaseTag pins the prometheus-community/postgres_exporter
// binary version the harness downloads. Bump intentionally when updating the
// head-to-head baseline.
const PostgresExporterReleaseTag = "v0.19.1"

// PostgresExporter is a running postgres_exporter subprocess owned by the test.
type PostgresExporter struct {
	// URL is the base URL for Prometheus scrapes (e.g. http://127.0.0.1:PORT).
	// Hit URL + "/metrics" to collect.
	URL string
	// Port is the TCP port the exporter is listening on.
	Port int

	cmd *exec.Cmd
}

// Stop terminates the exporter process and waits for it to exit. Safe to
// call more than once.
func (p *PostgresExporter) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = p.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		// The process ignored SIGINT — kill hard.
		_ = p.cmd.Process.Kill()
		<-done
	}
	p.cmd = nil
}

// StartPostgresExporter downloads (if needed) and boots a postgres_exporter
// binary pointed at the DSN, on an ephemeral port. The caller owns the DSN
// and is responsible for the Postgres instance's lifecycle — typically
// StartPG(t, "17.6") first, then pass its PgURI() or an equivalent string.
//
// DSN format follows libpq / lib/pq (postgres_exporter uses lib/pq under the
// hood) — both "postgres://..." URIs and "host=... user=..." key=value strings
// are accepted. For the trust-auth instance StartPG produces, the simplest
// form is:
//
//	fmt.Sprintf("host=127.0.0.1 port=%d user=postgres sslmode=disable", pg.Port)
//
// On non-Linux/amd64, t.Skip is called — matches the StartPG constraint.
func StartPostgresExporter(t testing.TB, dsn string) *PostgresExporter {
	t.Helper()

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skipf("testutil.StartPostgresExporter: pre-built binaries are Linux/amd64 only (have %s/%s)",
			runtime.GOOS, runtime.GOARCH)
	}

	binPath, err := ensurePostgresExporterBinary()
	if err != nil {
		t.Fatalf("testutil.StartPostgresExporter: fetching binary: %v", err)
	}

	port, err := pickPort()
	if err != nil {
		t.Fatalf("testutil.StartPostgresExporter: picking port: %v", err)
	}

	cmd := exec.Command(binPath,
		fmt.Sprintf("--web.listen-address=127.0.0.1:%d", port),
		// --auto-discover-databases defaults off already, but set explicitly
		// to make the command line self-documenting for reviewers.
		"--no-auto-discover-databases",
		// Silence the exporter's startup banner; we care about /metrics output
		// only and stdout/stderr spam confuses benchmark harness users.
		"--log.level=warn",
	)
	// DSN as env var — postgres_exporter picks it up; keeps credentials out
	// of `ps` output (good hygiene even for throwaway test DSNs).
	cmd.Env = append(os.Environ(), "DATA_SOURCE_NAME="+dsn)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatalf("testutil.StartPostgresExporter: starting: %v", err)
	}

	pe := &PostgresExporter{
		URL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		Port: port,
		cmd:  cmd,
	}

	if err := waitExporterReady(context.Background(), pe.URL); err != nil {
		pe.Stop()
		t.Fatalf("testutil.StartPostgresExporter: waiting for /metrics: %v", err)
	}

	t.Cleanup(pe.Stop)
	return pe
}

func postgresExporterCacheRoot() (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	// cacheRoot returns ~/.cache/pgxporter/postgres — we want
	// ~/.cache/pgxporter/postgres-exporter alongside it. Rebuild from its
	// parent so we share the XDG_CACHE_HOME handling.
	return filepath.Join(filepath.Dir(root), "postgres-exporter"), nil
}

func ensurePostgresExporterBinary() (string, error) {
	root, err := postgresExporterCacheRoot()
	if err != nil {
		return "", err
	}
	version := PostgresExporterReleaseTag // e.g. "v0.19.1"
	extractDir := filepath.Join(root, version)

	// Fast path: previously extracted. Walk because the tarball nests the
	// binary under a version-prefixed directory.
	if binPath, ok := findBinary(extractDir, "postgres_exporter"); ok {
		return binPath, nil
	}

	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", fmt.Errorf("postgres_exporter cache: mkdir: %w", err)
	}

	// Strip the leading "v" for the asset path — release v0.19.1 ships
	// postgres_exporter-0.19.1.linux-amd64.tar.gz.
	stripped := version
	if len(stripped) > 0 && stripped[0] == 'v' {
		stripped = stripped[1:]
	}
	url := fmt.Sprintf(
		"https://github.com/prometheus-community/postgres_exporter/releases/download/%s/postgres_exporter-%s.linux-amd64.tar.gz",
		version, stripped,
	)
	if err := downloadAndExtract(url, extractDir); err != nil {
		_ = os.RemoveAll(extractDir)
		return "", fmt.Errorf("postgres_exporter download: %w", err)
	}

	binPath, ok := findBinary(extractDir, "postgres_exporter")
	if !ok {
		return "", fmt.Errorf("postgres_exporter binary not found under %s after extract", extractDir)
	}
	return binPath, nil
}

// findBinary walks root looking for a regular executable file with the given
// name. Returns empty + false if missing. Handles the tarball layout which
// nests the binary inside postgres_exporter-X.Y.Z.linux-amd64/.
func findBinary(root, name string) (string, bool) {
	var found string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", false
	}
	return found, true
}

func waitExporterReady(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("waitExporterReady: %w", ctx.Err())
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/metrics", nil)
		resp, err := client.Do(req)
		if err == nil {
			body, _ := readAndDiscard(resp)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && body > 0 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waitExporterReady: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// readAndDiscard drains resp.Body and returns the byte count.
func readAndDiscard(resp *http.Response) (int64, error) {
	return io.Copy(io.Discard, resp.Body)
}
