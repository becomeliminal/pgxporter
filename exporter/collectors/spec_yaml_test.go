package collectors

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestParseSpecs_Happy(t *testing.T) {
	yaml := []byte(`
- subsystem: custom_rates
  sql: |
    SELECT current_database() AS database, key, count(*) AS c
    FROM business.rate_events
    GROUP BY key
  labels: [key]
  metrics:
    - name: events_total
      help: Events by rate key
      type: counter
      column: c
- subsystem: bloat
  namespace: pg
  min_pg_version: "14.0"
  sql: SELECT current_database() AS database, schemaname, bytes FROM monitoring.bloat
  labels: [schemaname]
  metrics:
    - name: bytes
      help: Table bloat in bytes
      column: bytes
`)
	specs, err := ParseSpecs(yaml)
	if err != nil {
		t.Fatalf("ParseSpecs: %v", err)
	}
	if got, want := len(specs), 2; got != want {
		t.Fatalf("specs: got %d, want %d", got, want)
	}

	// Spec 0: counter, default namespace.
	s0 := specs[0]
	if s0.Subsystem != "custom_rates" || s0.Namespace != "" {
		t.Errorf("spec[0] subsystem/namespace wrong: %+v", s0)
	}
	if len(s0.Metrics) != 1 || s0.Metrics[0].Type != prometheus.CounterValue {
		t.Errorf("spec[0] metrics wrong: %+v", s0.Metrics)
	}

	// Spec 1: gauge (default), explicit namespace, version-gated.
	s1 := specs[1]
	if s1.Namespace != "pg" {
		t.Errorf("spec[1] namespace: got %q", s1.Namespace)
	}
	if s1.MinPGVersion != [2]int{14, 0} {
		t.Errorf("spec[1] version: got %v", s1.MinPGVersion)
	}
	if s1.Metrics[0].Type != prometheus.GaugeValue {
		t.Errorf("spec[1] default type should be gauge, got %v", s1.Metrics[0].Type)
	}
}

func TestParseSpecs_VersionEncodings(t *testing.T) {
	cases := map[string][2]int{
		`- subsystem: a
  sql: SELECT 1
  min_pg_version: "17.6"
  metrics: [{name: m, column: c}]
`: {17, 6},
		`- subsystem: a
  sql: SELECT 1
  min_pg_version: [17, 6]
  metrics: [{name: m, column: c}]
`: {17, 6},
		`- subsystem: a
  sql: SELECT 1
  min_pg_version: "16"
  metrics: [{name: m, column: c}]
`: {16, 0},
	}
	for input, want := range cases {
		specs, err := ParseSpecs([]byte(input))
		if err != nil {
			t.Errorf("input %q: unexpected error: %v", input, err)
			continue
		}
		if specs[0].MinPGVersion != want {
			t.Errorf("input %q: got %v, want %v", input, specs[0].MinPGVersion, want)
		}
	}
}

func TestParseSpecs_Validation(t *testing.T) {
	cases := []string{
		`- sql: SELECT 1
  metrics: [{name: m, column: c}]
`, // missing subsystem
		`- subsystem: a
  metrics: [{name: m, column: c}]
`, // missing sql
		`- subsystem: a
  sql: SELECT 1
`, // missing metrics
		`- subsystem: a
  sql: SELECT 1
  metrics: [{column: c}]
`, // metric missing name
		`- subsystem: a
  sql: SELECT 1
  metrics: [{name: m, column: c, type: histogram}]
`, // invalid metric type
	}
	for _, input := range cases {
		if _, err := ParseSpecs([]byte(input)); err == nil {
			t.Errorf("expected error for input:\n%s", input)
		}
	}
}
