package collectors

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewSpecCollector_PanicsOnInvalid(t *testing.T) {
	tests := []struct {
		name string
		spec CollectorSpec
	}{
		{"missing subsystem", CollectorSpec{SQL: "x", Metrics: []MetricSpec{{Name: "n", Column: "c"}}}},
		{"missing SQL", CollectorSpec{Subsystem: "s", Metrics: []MetricSpec{{Name: "n", Column: "c"}}}},
		{"missing metrics", CollectorSpec{Subsystem: "s", SQL: "x"}},
		{"metric missing Name", CollectorSpec{Subsystem: "s", SQL: "x", Metrics: []MetricSpec{{Column: "c"}}}},
		{"metric missing Column", CollectorSpec{Subsystem: "s", SQL: "x", Metrics: []MetricSpec{{Name: "n"}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic, got none")
				}
			}()
			_ = NewSpecCollector(tc.spec, nil)
		})
	}
}

func TestNewSpecCollector_DescribeEmitsOnePerMetric(t *testing.T) {
	spec := CollectorSpec{
		Subsystem: "demo",
		SQL:       "SELECT 1",
		Labels:    []string{"name"},
		Metrics: []MetricSpec{
			{Name: "count_total", Help: "count", Type: prometheus.CounterValue, Column: "count"},
			{Name: "size_bytes", Help: "size", Column: "size"},
		},
	}
	c := NewSpecCollector(spec, nil)
	if got, want := len(drainDescs(c.Describe)), 2; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestCoerceFloat(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tests := []struct {
		in       any
		tsAsUnix bool
		want     float64
		ok       bool
	}{
		{int(42), false, 42, true},
		{int64(7), false, 7, true},
		{float64(3.14), false, 3.14, true},
		{now, true, 1700000000, true},
		{now, false, 0, false},
		{"nope", false, 0, false},
		{nil, false, 0, false},
	}
	for i, tc := range tests {
		got, ok := coerceFloat(tc.in, tc.tsAsUnix)
		if ok != tc.ok || got != tc.want {
			t.Errorf("case %d: coerceFloat(%v, %v) = (%v, %v), want (%v, %v)",
				i, tc.in, tc.tsAsUnix, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCoerceString(t *testing.T) {
	if got := coerceString("hello"); got != "hello" {
		t.Errorf("string: got %q", got)
	}
	if got := coerceString([]byte("bytes")); got != "bytes" {
		t.Errorf("[]byte: got %q", got)
	}
	if got := coerceString(nil); got != "" {
		t.Errorf("nil: got %q", got)
	}
	if got := coerceString(42); got != "42" {
		t.Errorf("int: got %q", got)
	}
}
