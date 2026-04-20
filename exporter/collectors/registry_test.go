package collectors

import (
	"slices"
	"testing"
)

func TestAvailableCollectors_IsSortedAndCoversRegistry(t *testing.T) {
	names := AvailableCollectors()
	if len(names) != len(collectorRegistry) {
		t.Errorf("AvailableCollectors len=%d, registry len=%d", len(names), len(collectorRegistry))
	}
	if !slices.IsSorted(names) {
		t.Errorf("AvailableCollectors should be sorted, got %v", names)
	}
}

func TestResolveCollectors_Defaults(t *testing.T) {
	got, err := ResolveCollectors(nil, nil, nil)
	if err != nil {
		t.Fatalf("ResolveCollectors: %v", err)
	}
	// Every default-enabled entry should be included; non-default ones skipped.
	wantN := 0
	for _, e := range collectorRegistry {
		if e.Default {
			wantN++
		}
	}
	if len(got) != wantN {
		t.Errorf("default resolution: got %d collectors, want %d", len(got), wantN)
	}
}

func TestResolveCollectors_EnabledOverridesDefault(t *testing.T) {
	// Only statements — everything else should drop even though it's default-on.
	got, err := ResolveCollectors(nil, []string{CollectorStatements}, nil)
	if err != nil {
		t.Fatalf("ResolveCollectors: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("enabled-only: got %d collectors, want 1", len(got))
	}
}

func TestResolveCollectors_DisabledSubtracts(t *testing.T) {
	got, err := ResolveCollectors(nil, nil, []string{CollectorActivity, CollectorLocks})
	if err != nil {
		t.Fatalf("ResolveCollectors: %v", err)
	}
	defaultN := 0
	for _, e := range collectorRegistry {
		if e.Default {
			defaultN++
		}
	}
	if len(got) != defaultN-2 {
		t.Errorf("disabled subtract: got %d collectors, want %d", len(got), defaultN-2)
	}
}

func TestResolveCollectors_DisabledWinsOverEnabled(t *testing.T) {
	got, _ := ResolveCollectors(nil,
		[]string{CollectorActivity, CollectorLocks},
		[]string{CollectorLocks},
	)
	if len(got) != 1 {
		t.Errorf("disabled-wins: got %d collectors, want 1 (activity only)", len(got))
	}
}

func TestResolveCollectors_UnknownNameIsNonFatal(t *testing.T) {
	got, err := ResolveCollectors(nil, []string{"not_a_thing"}, nil)
	if err == nil {
		t.Errorf("expected error for unknown collector name")
	}
	if len(got) != 0 {
		t.Errorf("unknown-only enabled should resolve to 0 collectors, got %d", len(got))
	}
}
