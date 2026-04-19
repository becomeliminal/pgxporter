package collectors

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// fakeCollector is a test Collector that records the context it was called
// with, so tests can verify deadline propagation.
type fakeCollector struct {
	desc    *prometheus.Desc
	scrapeF func(ctx context.Context, ch chan<- prometheus.Metric) error
}

func (c *fakeCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }
func (c *fakeCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	return c.scrapeF(ctx, ch)
}

// Verifies the Collector interface compiles with the ctx-aware signature and
// that the scrape receives a non-nil context. Guards the M1 interface change.
func TestCollectorInterfaceAcceptsContext(t *testing.T) {
	var c Collector = &fakeCollector{
		desc: prometheus.NewDesc("pg_stat_test", "help", nil, nil),
		scrapeF: func(ctx context.Context, ch chan<- prometheus.Metric) error {
			if ctx == nil {
				return errors.New("nil context")
			}
			return nil
		},
	}

	ch := make(chan prometheus.Metric, 8)
	if err := c.Scrape(context.Background(), ch); err != nil {
		t.Fatalf("Scrape: %v", err)
	}
}

// Verifies a cancelled context is observable by the scrape — the
// integration promise of the M1 change.
func TestScrapeObservesCancellation(t *testing.T) {
	var gotCtxErr error
	c := &fakeCollector{
		desc: prometheus.NewDesc("pg_stat_test", "help", nil, nil),
		scrapeF: func(ctx context.Context, _ chan<- prometheus.Metric) error {
			// Simulate a hung query: wait for ctx, don't time ourselves out.
			<-ctx.Done()
			gotCtxErr = ctx.Err()
			return gotCtxErr
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	ch := make(chan prometheus.Metric, 1)
	if err := c.Scrape(ctx, ch); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Scrape err = %v, want context.DeadlineExceeded", err)
	}
	if !errors.Is(gotCtxErr, context.DeadlineExceeded) {
		t.Fatalf("ctx.Err = %v inside scrape, want context.DeadlineExceeded", gotCtxErr)
	}
}
