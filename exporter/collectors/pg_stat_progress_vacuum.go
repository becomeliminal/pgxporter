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

	heapBlksTotal    *prometheus.Desc
	heapBlksScanned  *prometheus.Desc
	heapBlksVacuumed *prometheus.Desc
	indexVacuumCount *prometheus.Desc
}

// NewPgStatProgressVacuumCollector instantiates a new PgStatProgressVacuumCollector.
func NewPgStatProgressVacuumCollector(dbClients []*db.Client) *PgStatProgressVacuumCollector {
	labels := []string{"database", "datname", "relid", "phase"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, progressVacuumSubSystem, name), help, labels, nil)
	}
	return &PgStatProgressVacuumCollector{
		dbClients: dbClients,

		heapBlksTotal:    desc("heap_blks_total", "Total heap blocks in this relation"),
		heapBlksScanned:  desc("heap_blks_scanned", "Heap blocks scanned so far"),
		heapBlksVacuumed: desc("heap_blks_vacuumed", "Heap blocks vacuumed so far"),
		indexVacuumCount: desc("index_vacuum_count", "Number of index vacuum cycles completed"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgStatProgressVacuumCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.heapBlksTotal
	ch <- c.heapBlksScanned
	ch <- c.heapBlksVacuumed
	ch <- c.indexVacuumCount
}

// Scrape implements our Scraper interface.
func (c *PgStatProgressVacuumCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
	group, gctx := errgroup.WithContext(ctx)
	for _, dbClient := range c.dbClients {
		dbClient := dbClient
		group.Go(func() error { return c.scrape(gctx, dbClient, ch) })
	}
	if err := group.Wait(); err != nil {
		return fmt.Errorf("scraping: %w", err)
	}
	return nil
}

func (c *PgStatProgressVacuumCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgStatProgressVacuum(ctx)
	if err != nil {
		return fmt.Errorf("progress_vacuum stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgStatProgressVacuumCollector) emit(stats []*model.PgStatProgressVacuum, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.DatName.String,
			stat.RelID.String,
			stat.Phase.String,
		}
		emitInt := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Int64), labels...)
			}
		}
		emitInt(c.heapBlksTotal, stat.HeapBlksTotal)
		emitInt(c.heapBlksScanned, stat.HeapBlksScanned)
		emitInt(c.heapBlksVacuumed, stat.HeapBlksVacuumed)
		emitInt(c.indexVacuumCount, stat.IndexVacuumCount)
	}
}
