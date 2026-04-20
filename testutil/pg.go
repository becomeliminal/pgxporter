// Package testutil starts disposable Postgres servers for integration tests.
//
// It downloads pre-built Linux/amd64 binaries from the becomeliminal/postgres
// release artifacts (https://github.com/becomeliminal/postgres/releases), caches
// them under $XDG_CACHE_HOME (defaults to ~/.cache/pgxporter/postgres), and
// boots a postgres process on an ephemeral port.
//
// Compared to testcontainers or docker-compose:
//
//   - ~10MB download vs a 200MB+ Docker image.
//   - Sub-second startup on a warm cache — no container init overhead.
//   - No Docker daemon; works inside RBE workers without Docker-in-Docker.
//
// Platform: Linux/amd64 only. On other OSes, [StartPG] skips via [testing.T.Skip].
//
// Typical usage inside an integration test:
//
//	func TestMyCollector(t *testing.T) {
//	    pg := testutil.StartPG(t, "17.6")
//	    defer pg.Stop()
//	    // pg.DSN is a ready-to-use pgx-compatible connection string.
//	}
package testutil

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// BinaryReleaseTag is the becomeliminal/postgres release the harness downloads
// binaries from. Pinned here so upstream changes don't silently shift test
// behaviour; bump intentionally.
const BinaryReleaseTag = "v0.0.5"

// SupportedVersions lists the Postgres minor versions the release assets
// provide. Keep in sync with becomeliminal/postgres release notes when you
// bump BinaryReleaseTag.
var SupportedVersions = []string{
	"13.22",
	"14.19",
	"15.14",
	"16.10",
	"17.6",
	"18.0",
}

// PG is a running, disposable Postgres server owned by a test.
type PG struct {
	// DSN is a ready-to-use libpq / pgx connection string for the `postgres`
	// database as the superuser. Suitable for pgxpool.ParseConfig.
	DSN string
	// Host is the TCP host the server is listening on (always "127.0.0.1").
	Host string
	// Port is the TCP port the server is listening on.
	Port int
	// DataDir is the initdb data directory. Writable by the test, removed on Stop.
	DataDir string
	// Version is the Postgres version this server is running (e.g. "17.6").
	Version string

	cmd *exec.Cmd
}

// Stop terminates the postgres process, waits for it to exit, and removes the
// data directory. Safe to call more than once.
func (p *PG) Stop() {
	if p == nil {
		return
	}
	if p.cmd != nil && p.cmd.Process != nil {
		// postgres handles SIGTERM as a "smart shutdown" — wait for clients.
		// For tests that's fine because no clients remain.
		_ = p.cmd.Process.Signal(os.Interrupt)
		_ = p.cmd.Wait()
		p.cmd = nil
	}
	if p.DataDir != "" {
		_ = os.RemoveAll(p.DataDir)
		p.DataDir = ""
	}
}

// StartPG boots a Postgres server for the test. On non-Linux/amd64, it calls
// t.Skip. On any setup error, it fails the test (not a framework bug — we
// fail loudly rather than silently returning nil).
//
// The server's lifetime is bounded by the test: StartPG registers t.Cleanup
// to call Stop automatically, so callers don't strictly need defer pg.Stop()
// (though keeping it is harmless and keeps tests readable).
// StartPG takes testing.TB so both tests and benchmarks can spin up a PG.
func StartPG(t testing.TB, version string) *PG {
	t.Helper()

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skipf("testutil.StartPG: pre-built binaries are Linux/amd64 only (have %s/%s); use a local PG instead",
			runtime.GOOS, runtime.GOARCH)
	}
	if !isSupportedVersion(version) {
		t.Fatalf("testutil.StartPG: version %q not in SupportedVersions %v — bump becomeliminal/postgres release or adjust list",
			version, SupportedVersions)
	}

	binDir, err := ensureBinaries(version)
	if err != nil {
		t.Fatalf("testutil.StartPG: fetching PG %s binaries: %v", version, err)
	}

	dataDir := t.TempDir()
	if err := initDB(binDir, dataDir); err != nil {
		t.Fatalf("testutil.StartPG: initdb: %v", err)
	}

	port, err := pickPort()
	if err != nil {
		t.Fatalf("testutil.StartPG: picking port: %v", err)
	}

	cmd, err := startPostgres(binDir, dataDir, port)
	if err != nil {
		t.Fatalf("testutil.StartPG: starting postgres: %v", err)
	}

	pg := &PG{
		DSN:     fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable", port),
		Host:    "127.0.0.1",
		Port:    port,
		DataDir: dataDir,
		Version: version,
		cmd:     cmd,
	}

	if err := waitReady(context.Background(), binDir, port); err != nil {
		pg.Stop()
		t.Fatalf("testutil.StartPG: waiting for ready: %v", err)
	}

	t.Cleanup(pg.Stop)
	return pg
}

func isSupportedVersion(v string) bool {
	for _, s := range SupportedVersions {
		if s == v {
			return true
		}
	}
	return false
}

// cacheRoot returns the directory we extract binaries under. Honours
// XDG_CACHE_HOME, falling back to ~/.cache.
func cacheRoot() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "pgxporter", "postgres"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cacheRoot: %w", err)
	}
	return filepath.Join(home, ".cache", "pgxporter", "postgres"), nil
}

// ensureBinaries downloads + extracts the release tarball for version if not
// already cached, and returns the bin directory path.
func ensureBinaries(version string) (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	extractDir := filepath.Join(root, version)
	binDir := filepath.Join(extractDir, "bin")

	// Fast path: binaries already extracted.
	if stat, err := os.Stat(filepath.Join(binDir, "postgres")); err == nil && !stat.IsDir() {
		return binDir, nil
	}

	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", fmt.Errorf("ensureBinaries: mkdir: %w", err)
	}

	url := fmt.Sprintf("https://github.com/becomeliminal/postgres/releases/download/%s/psql-%s-linux_x86_64.tar.gz",
		BinaryReleaseTag, version)
	if err := downloadAndExtract(url, extractDir); err != nil {
		// Clean up a partial extraction so a retry can succeed.
		_ = os.RemoveAll(extractDir)
		return "", fmt.Errorf("ensureBinaries: %w", err)
	}

	if _, err := os.Stat(filepath.Join(binDir, "postgres")); err != nil {
		return "", fmt.Errorf("ensureBinaries: postgres binary not found after extract at %s", binDir)
	}
	return binDir, nil
}

func downloadAndExtract(url, destRoot string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: unexpected status %d from %s", resp.StatusCode, url)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("download: gunzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		// Tarballs are flat: paths start at "bin/postgres", "lib/...", etc.
		// Reject anything attempting to escape destRoot.
		name := hdr.Name
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(destRoot, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("tar mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("tar mkdir parent %s: %w", target, err)
			}
			mode := hdr.FileInfo().Mode().Perm()
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("tar create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return fmt.Errorf("tar write %s: %w", target, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("tar close %s: %w", target, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("tar mkdir parent (symlink) %s: %w", target, err)
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("tar symlink %s → %s: %w", target, hdr.Linkname, err)
			}
		}
	}
}

func initDB(binDir, dataDir string) error {
	// --auth=trust makes tests able to connect without a password.
	// --no-sync is fine for throwaway instances and speeds up initdb ~2x.
	cmd := exec.Command(
		filepath.Join(binDir, "initdb"),
		"--pgdata="+dataDir,
		"--username=postgres",
		"--auth=trust",
		"--encoding=UTF8",
		"--no-sync",
	)
	cmd.Env = pgEnv(binDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("initdb: %w\n%s", err, out)
	}
	return nil
}

func pickPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func startPostgres(binDir, dataDir string, port int) (*exec.Cmd, error) {
	cmd := exec.Command(
		filepath.Join(binDir, "postgres"),
		"-D", dataDir,
		"-p", fmt.Sprintf("%d", port),
		"-h", "127.0.0.1",
		// Quieter logs; tests don't need stderr spam.
		"-c", "log_min_messages=warning",
		"-c", "logging_collector=off",
		// Fewer fsyncs → faster tests. Data is discarded anyway.
		"-c", "fsync=off",
		"-c", "synchronous_commit=off",
		"-c", "full_page_writes=off",
	)
	cmd.Env = pgEnv(binDir)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("postgres start: %w", err)
	}
	return cmd, nil
}

// waitReady polls pg_isready until the server accepts connections or ctx deadlines.
func waitReady(ctx context.Context, binDir string, port int) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	isready := filepath.Join(binDir, "pg_isready")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("waitReady: %w", ctx.Err())
		}
		cmd := exec.CommandContext(ctx, isready, "-h", "127.0.0.1", "-p", fmt.Sprintf("%d", port), "-U", "postgres")
		cmd.Env = pgEnv(binDir)
		if err := cmd.Run(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waitReady: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// pgEnv returns the environment a Postgres binary needs to find its shared
// libraries, regardless of how the caller has LD_LIBRARY_PATH set up. Required
// because the becomeliminal/postgres tarballs ship their libs in lib/, not
// /usr/lib.
func pgEnv(binDir string) []string {
	libDir := filepath.Join(filepath.Dir(binDir), "lib")
	env := append(os.Environ(),
		"LD_LIBRARY_PATH="+libDir+string(os.PathListSeparator)+os.Getenv("LD_LIBRARY_PATH"),
	)
	return env
}
