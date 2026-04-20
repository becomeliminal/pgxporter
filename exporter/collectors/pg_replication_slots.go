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

	active            *prometheus.GaugeVec
	temporary         *prometheus.GaugeVec
	restartLsn        *prometheus.GaugeVec
	confirmedFlushLsn *prometheus.GaugeVec
	retainedWalBytes  *prometheus.GaugeVec
	safeWalSizeBytes  *prometheus.GaugeVec
	conflicting       *prometheus.GaugeVec
}

// NewPgReplicationSlotsCollector instantiates a new PgReplicationSlotsCollector.
func NewPgReplicationSlotsCollector(dbClients []*db.Client) *PgReplicationSlotsCollector {
	labels := []string{"database", "slot_name", "slot_type", "plugin", "datname", "wal_status"}
	gauge := gaugeFactoryRawPg(replicationSlotsSubSystem, labels)
	return &PgReplicationSlotsCollector{
		dbClients: dbClients,

		active:            gauge("active", "1 if the slot is currently active, 0 otherwise"),
		temporary:         gauge("temporary", "1 if the slot is temporary (dies with session), 0 otherwise"),
		restartLsn:        gauge("restart_lsn_bytes", "Oldest WAL location the slot requires, as a byte offset"),
		confirmedFlushLsn: gauge("confirmed_flush_lsn_bytes", "Logical-slot: last LSN confirmed flushed by the consumer"),
		retainedWalBytes:  gauge("retained_wal_bytes", "WAL bytes retained by this slot (pg_current_wal_lsn - restart_lsn); NULL on standby"),
		safeWalSizeBytes:  gauge("safe_wal_size_bytes", "WAL bytes remaining before the slot becomes 'lost' (PG 13+)"),
		conflicting:       gauge("conflicting", "1 if the slot is conflicting with recovery, 0 otherwise (PG 16+)"),
	}
}

// Describe implements the prometheus.Collector.
func (c *PgReplicationSlotsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.active.Describe(ch)
	c.temporary.Describe(ch)
	c.restartLsn.Describe(ch)
	c.confirmedFlushLsn.Describe(ch)
	c.retainedWalBytes.Describe(ch)
	c.safeWalSizeBytes.Describe(ch)
	c.conflicting.Describe(ch)
}

// Scrape implements our Scraper interface.
func (c *PgReplicationSlotsCollector) Scrape(ctx context.Context, ch chan<- prometheus.Metric) error {
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

func (c *PgReplicationSlotsCollector) collectInto(ch chan<- prometheus.Metric) {
	c.active.Collect(ch)
	c.temporary.Collect(ch)
	c.restartLsn.Collect(ch)
	c.confirmedFlushLsn.Collect(ch)
	c.retainedWalBytes.Collect(ch)
	c.safeWalSizeBytes.Collect(ch)
	c.conflicting.Collect(ch)
}

func (c *PgReplicationSlotsCollector) scrape(ctx context.Context, dbClient *db.Client) error {
	stats, err := dbClient.SelectPgReplicationSlots(ctx)
	if err != nil {
		return fmt.Errorf("replication_slots stats: %w", err)
	}
	c.emit(stats)
	return nil
}

func (c *PgReplicationSlotsCollector) emit(stats []*model.PgReplicationSlot) {
	for _, stat := range stats {
		labels := []string{
			stat.Database.String,
			stat.SlotName.String,
			stat.SlotType.String,
			stat.Plugin.String,
			stat.DatName.String,
			stat.WalStatus.String,
		}
		emitBool := func(vec *prometheus.GaugeVec, v pgtype.Bool) {
			if v.Valid {
				val := 0.0
				if v.Bool {
					val = 1.0
				}
				vec.WithLabelValues(labels...).Set(val)
			}
		}
		emitFloat := func(vec *prometheus.GaugeVec, v pgtype.Float8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(v.Float64)
			}
		}
		emitInt := func(vec *prometheus.GaugeVec, v pgtype.Int8) {
			if v.Valid {
				vec.WithLabelValues(labels...).Set(float64(v.Int64))
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
