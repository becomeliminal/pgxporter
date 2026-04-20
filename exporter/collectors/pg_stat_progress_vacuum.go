package collectors

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/becomeliminal/pgxporter/exporter/db"
	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// PgStatProgressVacuumCollector emits live VACUUM progress. Zero rows is
// the norm; metrics only appear while a worker is active.
//
// Labels: database, datname, relid, phase. The dead-tuple columns are
// intentionally skipped — PG 17 renamed them with changed semantics and
// we'll revisit when we version-gate the projection.
type PgStatProgressVacuumCollector struct {
	dbClients []*db.Client

	heapBlksTotal    *prometheus.GaugeVec
	heapBlksScanned  *prometheus.GaugeVec
	heapBlksVacuumed *prometheus.GaugeVec
	indexVacuumCount *prometheus.GaugeVec
}

// NewPgStatProgressVacuumCollector instantiates a new PgStatProgressVacuumCollector.
func NewPgStatProgressVacuumCollector(dbClients []*db.Client) *PgStatProgressVacuumCollector {
	labels := []string{"database", "datname", "relid", "phase"}
	gauge := gaugeFactory(progressVacuumSubSystem, labels)
	return &PgStatProgressVacuumCollector{
		dbClients: dbClients,

		heapBlksTotal:    gauge("heap_blks_total", "Total heap blocks in this relation"),
		heapBlksScanned:  gauge("heap_blks_scanned", "Heap blocks scanned so far"),
		heapBlksVacuumed: gauge("heap_blks_vacuumed", "Heap blocks vacuumed so far"),
		indexVacuumCount: gauge("index_vacuum_count", "Number of index vacuum cycles completed"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressVacuumCollector) Describe(ch chan<- *prometheus.Desc) {
	c.heapBlksTotal.Describe(ch)
	c.heapBlksScanned.Describe(ch)
	c.heapBlksVacuumed.Describe(ch)
	c.indexVacuumCount.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgStatProgressVacuumCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(gctx, dbClient) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	c.collectInto(ch)
	return nil
}

func (c *PgStatProgressVacuumCollector) collectInto(ch chan<- prometheus.Metric) {
	c.heapBlksTotal.Collect(ch)
	c.heapBlksScanned.Collect(ch)
	c.heapBlksVacuumed.Collect(ch)
	c.indexVacuumCount.Collect(ch)
}

func (c *PgStatProgressVacuumCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgStatProgressVacuum(ctx)
	if err != nil {
		return fmt.Errorf("progress_vacuum stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgStatProgressVacuumCollector) emit(stats []*model.PgStatProgressVacuum) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.DatName.String,
			stat.RelID.String,
			stat.Phase.String,
		}
		emitInt := func(vec *prometheus.GaugeVec, v pgtype.Int8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(float64(v.Int64))
			}
		}
		emitInt(c.heapBlksTotal, stat.HeapBlksTotal)
		emitInt(c.heapBlksScanned, stat.HeapBlksScanned)
		emitInt(c.heapBlksVacuumed, stat.HeapBlksVacuumed)
		emitInt(c.indexVacuumCount, stat.IndexVacuumCount)
	}
}
