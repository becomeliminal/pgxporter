package testutil

// Harness that runs the pgxporter reference binary (cmd/) as a subprocess
// for benchmarks that need apples-to-apples comparison with out-of-process
// competitors like prometheus-community/postgres_exporter. Running
// pgxporter in-process via httptest.Server measures its server-side
// allocations inside the test process, while a subprocess competitor's
// server-side allocations are invisible — that asymmetry makes any
// memory/alloc comparison across the two unfair. Putting pgxporter in a
// subprocess too restores symmetry: both are measured strictly from the
// client side.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// Pgxporter is a running pgxporter subprocess owned by the test.
type Pgxporter struct {
	// URL is the base URL for Prometheus scrapes (e.g. http://127.0.0.1:PORT).
	// Hit URL + "/metrics" to collect.
	URL string
	// Port is the TCP port the exporter is listening on.
	Port int

	cmd *exec.Cmd
}

// Stop terminates the exporter process and waits for it to exit. Safe to
// call more than once.
func (p *Pgxporter) Stop() {
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
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
	p.cmd = nil
}

// StartPgxporter builds (if needed) and boots the pgxporter reference
// binary from cmd/, pointed at dsn, on an ephemeral port. Registers
// t.Cleanup to kill the process when the test ends.
//
// The binary is compiled once per test-binary invocation and cached —
// subsequent calls in the same run reuse it.
func StartPgxporter(t testing.TB, dsn string) *Pgxporter {
	t.Helper()

	binPath, err := buildPgxporterBinary()
	if err != nil {
		t.Fatalf("testutil.StartPgxporter: building binary: %v", err)
	}

	port, err := pickPort()
	if err != nil {
		t.Fatalf("testutil.StartPgxporter: picking port: %v", err)
	}

	cmd := exec.Command(binPath,
		fmt.Sprintf("--web.listen-address=127.0.0.1:%d", port),
		"--log.level=error",
	)
	cmd.Env = append(os.Environ(), "DATA_SOURCE_NAME="+dsn)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("testutil.StartPgxporter: starting: %v", err)
	}

	pe := &Pgxporter{
		URL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		Port: port,
		cmd:  cmd,
	}

	if err := waitExporterReady(context.Background(), pe.URL); err != nil {
		pe.Stop()
		t.Fatalf("testutil.StartPgxporter: waiting for /metrics: %v\nsubprocess stderr:\n%s",
			err, stderr.String())
	}

	t.Cleanup(pe.Stop)
	return pe
}

var (
	pgxporterBuildOnce sync.Once
	pgxporterBinPath   string
	pgxporterBuildErr  error
)

// buildPgxporterBinary compiles cmd/ into a temp file once per process
// (sync.Once) and returns the path. Subsequent callers in the same test
// run share the binary.
func buildPgxporterBinary() (string, error) {
	pgxporterBuildOnce.Do(func() {
		// Find the repo root by walking up from this source file until we
		// see go.mod.
		_, thisFile, _, _ := runtime.Caller(0)
		root := filepath.Dir(thisFile)
		for {
			if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				pgxporterBuildErr = fmt.Errorf("could not locate go.mod ancestor of %s", thisFile)
				return
			}
			root = parent
		}

		tmp, err := os.CreateTemp("", "pgxporter-bin-*")
		if err != nil {
			pgxporterBuildErr = fmt.Errorf("tempfile: %w", err)
			return
		}
		_ = tmp.Close()

		cmd := exec.Command("go", "build", "-o", tmp.Name(), "./cmd")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.Remove(tmp.Name())
			pgxporterBuildErr = fmt.Errorf("go build ./cmd failed: %w\n%s", err, out)
			return
		}
		pgxporterBinPath = tmp.Name()
	})
	return pgxporterBinPath, pgxporterBuildErr
}
