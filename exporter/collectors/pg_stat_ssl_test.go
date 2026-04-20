package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

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
		{
			Database:    text("postgres"),
			SSL:         pgtype.Bool{Bool: true, Valid: true},
			Version:     text("TLSv1.3"),
			Cipher:      text("TLS_AES_256_GCM_SHA384"),
			Bits:        int8v(256),
			Connections: int8v(7),
		},
		{
			Database:    text("postgres"),
			SSL:         pgtype.Bool{Bool: false, Valid: true},
			Version:     text(""),
			Cipher:      text(""),
			Bits:        int8v(0),
			Connections: int8v(3),
		},
	}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 2; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatSSLCollector_Emit_SkipsInvalid(t *testing.T) {
	c := NewPgStatSSLCollector(nil)
	// ssl NULL → skip; connections NULL → skip.
	stats := []*model.PgStatSSL{
		{Database: text("postgres"), Connections: int8v(1)}, // ssl invalid
		{Database: text("postgres"), SSL: pgtype.Bool{Bool: true, Valid: true}}, // connections invalid
	}
	c.emit(stats)
	ms := drainMetrics(c.collectInto)
	if got, want := len(ms), 0; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}
