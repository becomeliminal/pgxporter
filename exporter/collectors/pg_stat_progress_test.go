package collectors

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

func TestPgStatProgressAnalyzeCollector_DescribeEmit(t *testing.T) {
	c := NewPgStatProgressAnalyzeCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 6; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
	stats := []*model.PgStatProgressAnalyze{{
		Database: text("postgres"), DatName: text("postgres"), RelID: text("16384"), Phase: text("acquiring sample rows"),
		SampleBlksTotal: int8v(100), SampleBlksScanned: int8v(40),
		ExtStatsTotal: int8v(2), ExtStatsComputed: int8v(1),
		ChildTablesTotal: int8v(4), ChildTablesDone: int8v(2),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 6; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatProgressBasebackupCollector_DescribeEmit(t *testing.T) {
	c := NewPgStatProgressBasebackupCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 4; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
	stats := []*model.PgStatProgressBasebackup{{
		Database: text("postgres"), Phase: text("streaming database files"),
		BackupTotalBytes: int8v(1_000_000_000), BackupStreamedBytes: int8v(400_000_000),
		TablespacesTotal: int8v(3), TablespacesStreamed: int8v(1),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 4; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatProgressCopyCollector_DescribeEmit(t *testing.T) {
	c := NewPgStatProgressCopyCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 4; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
	stats := []*model.PgStatProgressCopy{{
		Database: text("postgres"), DatName: text("postgres"), RelID: text("16384"),
		Command: text("COPY FROM"), Type: text("FILE"),
		BytesTotal: int8v(1024), BytesProcessed: int8v(512),
		TuplesProcessed: int8v(10), TuplesExcluded: int8v(0),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 4; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatProgressCreateIndexCollector_DescribeEmit(t *testing.T) {
	c := NewPgStatProgressCreateIndexCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 8; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
	stats := []*model.PgStatProgressCreateIndex{{
		Database: text("postgres"), DatName: text("postgres"),
		RelID: text("16384"), IndexRelID: text("16400"),
		Command: text("CREATE INDEX"), Phase: text("building index"),
		LockersTotal: int8v(0), LockersDone: int8v(0),
		BlocksTotal: int8v(100), BlocksDone: int8v(50),
		TuplesTotal: int8v(1000), TuplesDone: int8v(500),
		PartitionsTotal: int8v(0), PartitionsDone: int8v(0),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 8; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatProgressClusterCollector_DescribeEmit(t *testing.T) {
	c := NewPgStatProgressClusterCollector(nil)
	if got, want := len(drainDescs(c.Describe)), 5; got != want {
		t.Errorf("Describe emitted %d, want %d", got, want)
	}
	stats := []*model.PgStatProgressCluster{{
		Database: text("postgres"), DatName: text("postgres"), RelID: text("16384"),
		Command: text("VACUUM FULL"), Phase: text("seq scanning heap"),
		HeapTuplesScanned: int8v(1000), HeapTuplesWritten: int8v(900),
		HeapBlksTotal: int8v(200), HeapBlksScanned: int8v(150),
		IndexRebuildCount: int8v(2),
	}}
	ms := drainMetrics(func(ch chan<- prometheus.Metric) { c.emit(stats, ch) })
	if got, want := len(ms), 5; got != want {
		t.Errorf("emit produced %d metrics, want %d", got, want)
	}
}

func TestPgStatProgressCollectors_EmptyEmitsNothing(t *testing.T) {
	a := NewPgStatProgressAnalyzeCollector(nil)
	b := NewPgStatProgressBasebackupCollector(nil)
	c := NewPgStatProgressCopyCollector(nil)
	ci := NewPgStatProgressCreateIndexCollector(nil)
	cl := NewPgStatProgressClusterCollector(nil)
	for _, emit := range []func(chan<- prometheus.Metric){
		func(ch chan<- prometheus.Metric) { a.emit(nil, ch) },
		func(ch chan<- prometheus.Metric) { b.emit(nil, ch) },
		func(ch chan<- prometheus.Metric) { c.emit(nil, ch) },
		func(ch chan<- prometheus.Metric) { ci.emit(nil, ch) },
		func(ch chan<- prometheus.Metric) { cl.emit(nil, ch) },
	} {
		if got := len(drainMetrics(emit)); got != 0 {
			t.Errorf("empty progress rows emitted %d metrics, want 0", got)
		}
	}
}
