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

// PgReplicationSlotsCollector collects per-slot state from pg_replication_slots.
// Both physical and logical slots are emitted in the same metric family,
// distinguished by the slot_type label.
//
// Labels: database, slot_name, slot_type, plugin, datname, wal_status.
type PgReplicationSlotsCollector struct {
	dbClients []*db.Client

	active            *prometheus.Desc
	temporary         *prometheus.Desc
	restartLsn        *prometheus.Desc
	confirmedFlushLsn *prometheus.Desc
	retainedWalBytes  *prometheus.Desc
	safeWalSizeBytes  *prometheus.Desc
	conflicting       *prometheus.Desc
}

// NewPgReplicationSlotsCollector instantiates a new PgReplicationSlotsCollector.
func NewPgReplicationSlotsCollector(dbClients []*db.Client) *PgReplicationSlotsCollector {
	labels := []string{"database", "slot_name", "slot_type", "plugin", "datname", "wal_status"}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespaceRawPg, replicationSlotsSubSystem, name), help, labels, nil)
	}
	return &PgReplicationSlotsCollector{
		dbClients: dbClients,

		active:            desc("active", "1 if the slot is currently active, 0 otherwise"),
		temporary:         desc("temporary", "1 if the slot is temporary (dies with session), 0 otherwise"),
		restartLsn:        desc("restart_lsn_bytes", "Oldest WAL location the slot requires, as a byte offset"),
		confirmedFlushLsn: desc("confirmed_flush_lsn_bytes", "Logical-slot: last LSN confirmed flushed by the consumer"),
		retainedWalBytes:  desc("retained_wal_bytes", "WAL bytes retained by this slot (pg_current_wal_lsn - restart_lsn); NULL on standby"),
		safeWalSizeBytes:  desc("safe_wal_size_bytes", "WAL bytes remaining before the slot becomes 'lost' (PG 13+)"),
		conflicting:       desc("conflicting", "1 if the slot is conflicting with recovery, 0 otherwise (PG 16+)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgReplicationSlotsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.active
	ch <- c.temporary
	ch <- c.restartLsn
	ch <- c.confirmedFlushLsn
	ch <- c.retainedWalBytes
	ch <- c.safeWalSizeBytes
	ch <- c.conflicting
}

// Scrape implements our Scraper interface.
func (c *PgReplicationSlotsCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgReplicationSlotsCollector) scrape(ctx context.Context, dbClient *db.Client, ch chan<- prometheus.Metric) error {
	stats, err := dbClient.SelectPgReplicationSlots(ctx)
	if err != nil {
		return fmt.Errorf("replication_slots stats: %w", err)
	}
	c.emit(stats, ch)
	return nil
}

func (c *PgReplicationSlotsCollector) emit(stats []*model.PgReplicationSlot, ch chan<- prometheus.Metric) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.SlotName.String,
			stat.SlotType.String,
			stat.Plugin.String,
			stat.DatName.String,
			stat.WalStatus.String,
		}
		emitBool := func(desc *prometheus.Desc, v pgtype.Bool) {
			if v.Valid {
				val := 0.0
				if v.Bool {
					val = 1.0
				}
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, labels...)
			}
		}
		emitFloat := func(desc *prometheus.Desc, v pgtype.Float8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v.Float64, labels...)
			}
		}
		emitInt := func(desc *prometheus.Desc, v pgtype.Int8) {
			if v.Valid {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(v.Int64), labels...)
			}
		}
		emitBool(c.active, stat.Active)
		emitBool(c.temporary, stat.Temporary)
		emitFloat(c.restartLsn, stat.RestartLsnBytes)
		emitFloat(c.confirmedFlushLsn, stat.ConfirmedFlushLsn)
		emitFloat(c.retainedWalBytes, stat.RetainedWalBytes)
		emitInt(c.safeWalSizeBytes, stat.SafeWalSizeBytes)
		emitBool(c.conflicting, stat.Conflicting)
	}
}
