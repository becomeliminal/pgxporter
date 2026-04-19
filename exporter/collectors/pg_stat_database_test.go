package collectors

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatDatabaseCollector_Describe(t *testing.T) {
	c := NewPgStatDatabaseCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 26; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
}

func TestPgStatDatabaseCollector_Emit(t *testing.T) {
	c := NewPgStatDatabaseCollector(nil)

	// Fully-populated row includes PG 12+ and PG 14+ columns.
	full := &model.PgStatDatabase{
		Database:              text("postgres"),
		DatName:               text("postgres"),
		NumBackends:           int8v(5),
		XactCommit:            int8v(100),
		XactRollback:          int8v(2),
		BlksRead:              int8v(500),
		BlksHit:               int8v(10000),
		TupReturned:           int8v(20000),
		TupFetched:            int8v(1500),
		TupInserted:           int8v(50),
		TupUpdated:            int8v(30),
		TupDeleted:            int8v(5),
		Conflicts:             int8v(0),
		TempFiles:             int8v(1),
		TempBytes:             int8v(1024),
		Deadlocks:             int8v(0),
		BlkReadTime:           floatv(100.0), // ms
		BlkWriteTime:          floatv(50.0),
		ChecksumFailures:      int8v(0),
		SessionTime:           floatv(60_000.0),
		ActiveTime:            floatv(30_000.0),
		IdleInTransactionTime: floatv(5_000.0),
		Sessions:              int8v(10),
		SessionsAbandoned:     int8v(1),
		SessionsFatal:         int8v(0),
		SessionsKilled:        int8v(0),
	}

	// Partitioned / template-like shape — mostly NULL, except identifiers.
	partial := &model.PgStatDatabase{
		Database: text("postgres"),
		DatName:  text("template1"),
	}

	t.Run("full row emits most metrics", func(t *testing.T) {
		ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit([]*model.PgStatDatabase{full}, ch) })
		// Exact count depends on which fields are Valid. 26 possible; in this
		// fixture ChecksumLastFailure and StatsReset are NULL → 24.
		if got := len(ms); got < 20 {
			t.Errorf("full row emitted only %d metrics, want >= 20", got)
		}
	})

	t.Run("NULL row emits nothing", func(t *testing.T) {
		empty := &model.PgStatDatabase{Database: text("postgres"), DatName: text("x")}
		empty.XactCommit = pgtype.Int8{}
		ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit([]*model.PgStatDatabase{empty}, ch) })
		if got := len(ms); got != 0 {
			t.Errorf("all-NULL row emitted %d metrics, want 0", got)
		}
	})

	t.Run("partial row only emits valid fields", func(t *testing.T) {
		ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit([]*model.PgStatDatabase{partial}, ch) })
		if got := len(ms); got != 0 {
			t.Errorf("partial row emitted %d metrics, want 0 (no valid counters)", got)
		}
	})
}
