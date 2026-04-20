package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatSSLCollector_Describe(t *testing.T) {
	c := NewPgStatSSLCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 1; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatSSLCollector_Emit(t *testing.T) {
	c := NewPgStatSSLCollector(nil)
	stats := []*model.PgStatSSL{
		{Database: text("postgres"), SSL: pgtype.Bool{Bool: true, Valid: true}, Version: text("TLSv1.3"), Cipher: text("TLS_AES_256_GCM_SHA384"), Bits: int8v(256), Count: int8v(12)},
		{Database: text("postgres"), SSL: pgtype.Bool{Bool: false, Valid: true}, Count: int8v(4)},
	}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 2; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatSSLCollector_EmitSkipsNullCount(t *testing.T) {
	c := NewPgStatSSLCollector(nil)
	stats := []*model.PgStatSSL{{
		Database: text("postgres"), SSL: pgtype.Bool{Bool: true, Valid: true},
		Count: pgtype.Int8{},
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got := len(ms); got != 0 {
		t.Errorf("NULL count should be skipped, got %d metrics", got)
	}
}
