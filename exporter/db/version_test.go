package db

import (
	"sync"
	"testing"
)

func TestAtLeast(t *testing.T) {
	tests := []struct {
		name         string
		versionNum   int
		wantMajor    int
		wantMinor    int
		expectResult bool
	}{
		{"zero version → always false (14.0)", 0, 14, 0, false},
		{"zero version → always false (9.0)", 0, 9, 0, false},
		{"PG 16.4 at least 14.0", 160004, 14, 0, true},
		{"PG 16.4 at least 16.0", 160004, 16, 0, true},
		{"PG 16.4 at least 16.4", 160004, 16, 4, true},
		{"PG 16.4 at least 16.5", 160004, 16, 5, false},
		{"PG 16.4 at least 17.0", 160004, 17, 0, false},
		{"PG 13.22 at least 13.0", 130022, 13, 0, true},
		{"PG 13.22 at least 14.0", 130022, 14, 0, false},
		{"PG 17.6 at least 17.0", 170006, 17, 0, true},
		{"PG 18.0 at least 17.0", 180000, 17, 0, true},
		{"PG 18.0 at least 18.0", 180000, 18, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{ServerVersionNum: tc.versionNum}
			got := c.AtLeast(tc.wantMajor, tc.wantMinor)
			if got != tc.expectResult {
				t.Errorf("Client{ServerVersionNum: %d}.AtLeast(%d, %d) = %v, want %v",
					tc.versionNum, tc.wantMajor, tc.wantMinor, got, tc.expectResult)
			}
		})
	}
}

// TestAtLeastNoPoolNoPanic guards the lazy re-probe path: AtLeast must
// short-circuit when the pool is nil even with ServerVersionNum == 0,
// so unit tests (and any caller that constructs a bare Client) do not
// panic.
func TestAtLeastNoPoolNoPanic(t *testing.T) {
	c := &Client{}
	if c.AtLeast(17, 0) {
		t.Error("AtLeast on bare Client returned true; want false")
	}
}

// TestAtLeastConcurrentNoRace exercises AtLeast from many goroutines
// against a Client with a known version (no re-probe path triggered)
// to assert it remains race-free under concurrent reads. Pair with
// `go test -race`.
func TestAtLeastConcurrentNoRace(t *testing.T) {
	c := &Client{ServerVersionNum: 170006}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = c.AtLeast(17, 0)
			}
		}()
	}
	wg.Wait()
}
