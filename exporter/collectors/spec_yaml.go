package collectors

import (
	"fmt"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
)

// yamlCollectorSpec is the YAML-serialisable mirror of CollectorSpec.
// prometheus.ValueType becomes a string ("counter" | "gauge") so
// operators aren't writing Go enum values in config.
type yamlCollectorSpec struct {
	Subsystem    string           `yaml:"subsystem"`
	Namespace    string           `yaml:"namespace,omitempty"`
	SQL          string           `yaml:"sql"`
	MinPGVersion yamlPGVersion    `yaml:"min_pg_version,omitempty"`
	Labels       []string         `yaml:"labels,omitempty"`
	Metrics      []yamlMetricSpec `yaml:"metrics"`
}

type yamlMetricSpec struct {
	Name            string  `yaml:"name"`
	Help            string  `yaml:"help"`
	Type            string  `yaml:"type,omitempty"` // "counter" | "gauge" | ""
	Column          string  `yaml:"column"`
	Scale           float64 `yaml:"scale,omitempty"`
	TimestampAsUnix bool    `yaml:"timestamp_as_unix,omitempty"`
}

// yamlPGVersion accepts either "17.0" or [17, 0] in YAML.
type yamlPGVersion [2]int

func (v *yamlPGVersion) UnmarshalYAML(node *yaml.Node) error {
	// Try the sequence form first.
	if node.Kind == yaml.SequenceNode {
		var pair [2]int
		if err := node.Decode(&pair); err != nil {
			return err
		}
		*v = yamlPGVersion(pair)
		return nil
	}
	// Fall back to "major.minor" string.
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	var major, minor int
	_, err := fmt.Sscanf(s, "%d.%d", &major, &minor)
	if err != nil {
		// Allow bare "17" too.
		if _, err2 := fmt.Sscanf(s, "%d", &major); err2 != nil {
			return fmt.Errorf("min_pg_version %q: expected major.minor, [major, minor], or bare major: %w", s, err)
		}
	}
	*v = yamlPGVersion{major, minor}
	return nil
}

// LoadSpecsFromFile reads a YAML file listing zero or more
// CollectorSpecs and returns them in internal form. The format is
// intentionally forgiving on details that vary across operators
// (min_pg_version accepts multiple encodings) but strict on the shape
// that makes collectors work (subsystem/sql/metrics all required).
//
// Example YAML:
//
//   - subsystem: custom_rates
//     sql: |
//     SELECT current_database() AS database,
//     key,
//     count(*) AS c
//     FROM business.rate_events
//     GROUP BY key
//     labels: [key]
//     metrics:
//   - name: events_total
//     help: "Events by rate key"
//     type: counter
//     column: c
//   - subsystem: bloat
//     min_pg_version: "14.0"
//     sql: |
//     SELECT current_database() AS database, schemaname, relname,
//     bytes_bloated
//     FROM monitoring.table_bloat
//     labels: [schemaname, relname]
//     metrics:
//   - name: bytes
//     help: Estimated table bloat in bytes
//     column: bytes_bloated
func LoadSpecsFromFile(path string) ([]CollectorSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseSpecs(data)
}

// ParseSpecs parses a YAML byte slice into CollectorSpecs. Exposed for
// tests so they don't have to round-trip through the filesystem.
func ParseSpecs(data []byte) ([]CollectorSpec, error) {
	var raw []yamlCollectorSpec
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	specs := make([]CollectorSpec, 0, len(raw))
	for i, y := range raw {
		s, err := y.toInternal()
		if err != nil {
			return nil, fmt.Errorf("spec[%d] (%s): %w", i, y.Subsystem, err)
		}
		specs = append(specs, s)
	}
	return specs, nil
}

func (y yamlCollectorSpec) toInternal() (CollectorSpec, error) {
	if y.Subsystem == "" {
		return CollectorSpec{}, fmt.Errorf("subsystem is required")
	}
	if y.SQL == "" {
		return CollectorSpec{}, fmt.Errorf("sql is required")
	}
	if len(y.Metrics) == 0 {
		return CollectorSpec{}, fmt.Errorf("metrics must contain at least one entry")
	}
	metrics := make([]MetricSpec, 0, len(y.Metrics))
	for i, m := range y.Metrics {
		if m.Name == "" || m.Column == "" {
			return CollectorSpec{}, fmt.Errorf("metrics[%d]: name and column are required", i)
		}
		var vtype prometheus.ValueType
		switch m.Type {
		case "", "gauge":
			vtype = prometheus.GaugeValue
		case "counter":
			vtype = prometheus.CounterValue
		default:
			return CollectorSpec{}, fmt.Errorf("metrics[%d].type: must be counter or gauge, got %q", i, m.Type)
		}
		metrics = append(metrics, MetricSpec{
			Name:            m.Name,
			Help:            m.Help,
			Type:            vtype,
			Column:          m.Column,
			Scale:           m.Scale,
			TimestampAsUnix: m.TimestampAsUnix,
		})
	}
	return CollectorSpec{
		Subsystem:    y.Subsystem,
		Namespace:    y.Namespace,
		SQL:          y.SQL,
		MinPGVersion: [2]int(y.MinPGVersion),
		Labels:       y.Labels,
		Metrics:      metrics,
	}, nil
}
