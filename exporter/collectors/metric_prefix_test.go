package collectors

import (
	"strings"
	"testing"
)

func TestSetMetricPrefix(t *testing.T) {
	// Restore default after the test so it doesn't leak into other tests.
	t.Cleanup(func() { SetMetricPrefix(MetricPrefixPgStat) })

	SetMetricPrefix(MetricPrefixPg)
	c := NewPgStatDatabaseCollector(nil)
	descs := drainDescs(c.Describe)
	if len(descs) == 0 {
		t.Fatal("expected Describe to emit descs")
	}
	// prometheus.Desc.String() looks like:
	//   Desc{fqName: "pg_database_xact_commit_total", ...}
	// so we assert the fqName prefix.
	s := descs[0].String()
	if !strings.Contains(s, `fqName: "pg_database_`) {
		t.Errorf("prefix=pg: expected fqName starting pg_database_, got %s", s)
	}

	SetMetricPrefix(MetricPrefixPgStat)
	c = NewPgStatDatabaseCollector(nil)
	descs = drainDescs(c.Describe)
	s = descs[0].String()
	if !strings.Contains(s, `fqName: "pg_stat_database_`) {
		t.Errorf("prefix=pg_stat: expected fqName starting pg_stat_database_, got %s", s)
	}
}

func TestSetMetricPrefix_EmptyFallsBackToDefault(t *testing.T) {
	t.Cleanup(func() { SetMetricPrefix(MetricPrefixPgStat) })
	SetMetricPrefix("")
	if namespace != "pg_stat" {
		t.Errorf("empty prefix should fall back to pg_stat, got %q", namespace)
	}
}
