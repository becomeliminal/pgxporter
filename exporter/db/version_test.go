package db

import "testing"

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
