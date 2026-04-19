package model

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// PgStatDatabase contains cluster-health stats per database.
//
// Version-gated columns:
//   - ChecksumFailures, ChecksumLastFailure: PG 12+
//   - Session-time / sessions counters: PG 14+
type PgStatDatabase struct {
	Database              pgtype.Text        `db:"database"`
	DatID                 pgtype.Int8        `db:"datid"`
	DatName               pgtype.Text        `db:"datname"`
	NumBackends           pgtype.Int8        `db:"numbackends"`
	XactCommit            pgtype.Int8        `db:"xact_commit"`
	XactRollback          pgtype.Int8        `db:"xact_rollback"`
	BlksRead              pgtype.Int8        `db:"blks_read"`
	BlksHit               pgtype.Int8        `db:"blks_hit"`
	TupReturned           pgtype.Int8        `db:"tup_returned"`
	TupFetched            pgtype.Int8        `db:"tup_fetched"`
	TupInserted           pgtype.Int8        `db:"tup_inserted"`
	TupUpdated            pgtype.Int8        `db:"tup_updated"`
	TupDeleted            pgtype.Int8        `db:"tup_deleted"`
	Conflicts             pgtype.Int8        `db:"conflicts"`
	TempFiles             pgtype.Int8        `db:"temp_files"`
	TempBytes             pgtype.Int8        `db:"temp_bytes"`
	Deadlocks             pgtype.Int8        `db:"deadlocks"`
	ChecksumFailures      pgtype.Int8        `db:"checksum_failures"`     // PG 12+
	ChecksumLastFailure   pgtype.Timestamptz `db:"checksum_last_failure"` // PG 12+
	BlkReadTime           pgtype.Float8      `db:"blk_read_time"`
	BlkWriteTime          pgtype.Float8      `db:"blk_write_time"`
	SessionTime           pgtype.Float8      `db:"session_time"`             // PG 14+
	ActiveTime            pgtype.Float8      `db:"active_time"`              // PG 14+
	IdleInTransactionTime pgtype.Float8      `db:"idle_in_transaction_time"` // PG 14+
	Sessions              pgtype.Int8        `db:"sessions"`                 // PG 14+
	SessionsAbandoned     pgtype.Int8        `db:"sessions_abandoned"`       // PG 14+
	SessionsFatal         pgtype.Int8        `db:"sessions_fatal"`           // PG 14+
	SessionsKilled        pgtype.Int8        `db:"sessions_killed"`          // PG 14+
	StatsReset            pgtype.Timestamptz `db:"stats_reset"`
}

// PgStatBgwriter contains background-writer statistics.
//
// PG 17 split most of this view into pg_stat_checkpointer. We keep one
// struct and version-gate the projection so the remaining PG < 17 columns
// are zero-valued on newer servers (and vice versa).
type PgStatBgwriter struct {
	Database        pgtype.Text        `db:"database"`
	BuffersClean    pgtype.Int8        `db:"buffers_clean"`
	MaxwrittenClean pgtype.Int8        `db:"maxwritten_clean"`
	BuffersAlloc    pgtype.Int8        `db:"buffers_alloc"`
	StatsReset      pgtype.Timestamptz `db:"stats_reset"`

	// PG < 17 only — moved to pg_stat_checkpointer in PG 17.
	CheckpointsTimed    pgtype.Int8   `db:"checkpoints_timed"`
	CheckpointsReq      pgtype.Int8   `db:"checkpoints_req"`
	CheckpointWriteTime pgtype.Float8 `db:"checkpoint_write_time"`
	CheckpointSyncTime  pgtype.Float8 `db:"checkpoint_sync_time"`
	BuffersCheckpoint   pgtype.Int8   `db:"buffers_checkpoint"`
	BuffersBackend      pgtype.Int8   `db:"buffers_backend"`
	BuffersBackendFsync pgtype.Int8   `db:"buffers_backend_fsync"`
}

// PgStatCheckpointer contains checkpoint activity.
//
// New view in PG 17 (split out from pg_stat_bgwriter). On pre-17 servers
// the dedicated collector returns an empty result and emits no metrics.
type PgStatCheckpointer struct {
	Database           pgtype.Text        `db:"database"`
	NumTimed           pgtype.Int8        `db:"num_timed"`
	NumRequested       pgtype.Int8        `db:"num_requested"`
	RestartpointsTimed pgtype.Int8        `db:"restartpoints_timed"`
	RestartpointsReq   pgtype.Int8        `db:"restartpoints_req"`
	RestartpointsDone  pgtype.Int8        `db:"restartpoints_done"`
	WriteTime          pgtype.Float8      `db:"write_time"` // milliseconds
	SyncTime           pgtype.Float8      `db:"sync_time"`  // milliseconds
	BuffersWritten     pgtype.Int8        `db:"buffers_written"`
	StatsReset         pgtype.Timestamptz `db:"stats_reset"`
}

// PgStatArchiver contains WAL-archiving statistics (cluster-wide, single row).
//
// Available on all supported PG versions. The last_archived_wal / last_failed_wal
// filenames are intentionally omitted — they're high-cardinality text and not
// useful as metric values; users wanting them can query the view directly.
type PgStatArchiver struct {
	Database         pgtype.Text        `db:"database"`
	ArchivedCount    pgtype.Int8        `db:"archived_count"`
	LastArchivedTime pgtype.Timestamptz `db:"last_archived_time"`
	FailedCount      pgtype.Int8        `db:"failed_count"`
	LastFailedTime   pgtype.Timestamptz `db:"last_failed_time"`
	StatsReset       pgtype.Timestamptz `db:"stats_reset"`
}

// PgStatReplication contains one row per downstream replica connected to the
// primary. On a standby the view is empty (or contains cascading downstream
// replicas). The LSN-diff columns are computed against pg_current_wal_lsn()
// and gated with CASE WHEN pg_is_in_recovery() so standbys don't hit the
// "recovery is in progress" error; on a standby they remain NULL.
type PgStatReplication struct {
	Database         pgtype.Text   `db:"database"`
	Pid              pgtype.Int8   `db:"pid"`
	ApplicationName  pgtype.Text   `db:"application_name"`
	ClientAddr       pgtype.Text   `db:"client_addr"`
	State            pgtype.Text   `db:"state"`
	SyncState        pgtype.Text   `db:"sync_state"`
	SentLagBytes     pgtype.Float8 `db:"sent_lag_bytes"`
	WriteLagBytes    pgtype.Float8 `db:"write_lag_bytes"`
	FlushLagBytes    pgtype.Float8 `db:"flush_lag_bytes"`
	ReplayLagBytes   pgtype.Float8 `db:"replay_lag_bytes"`
	WriteLagSeconds  pgtype.Float8 `db:"write_lag_seconds"`
	FlushLagSeconds  pgtype.Float8 `db:"flush_lag_seconds"`
	ReplayLagSeconds pgtype.Float8 `db:"replay_lag_seconds"`
}

// PgStatWalReceiver contains WAL-receiver stats on a standby (one row per
// active walreceiver). Empty on primaries. LSN columns are emitted as
// byte offsets (pg_wal_lsn_diff against '0/0') so downstream dashboards
// can diff across receivers.
type PgStatWalReceiver struct {
	Database             pgtype.Text        `db:"database"`
	Pid                  pgtype.Int8        `db:"pid"`
	Status               pgtype.Text        `db:"status"`
	ReceiveStartLsnBytes pgtype.Float8      `db:"receive_start_lsn_bytes"`
	WrittenLsnBytes      pgtype.Float8      `db:"written_lsn_bytes"`
	FlushedLsnBytes      pgtype.Float8      `db:"flushed_lsn_bytes"`
	LatestEndLsnBytes    pgtype.Float8      `db:"latest_end_lsn_bytes"`
	LastMsgSendTime      pgtype.Timestamptz `db:"last_msg_send_time"`
	LastMsgReceiptTime   pgtype.Timestamptz `db:"last_msg_receipt_time"`
	LatestEndTime        pgtype.Timestamptz `db:"latest_end_time"`
	SlotName             pgtype.Text        `db:"slot_name"`
	SenderHost           pgtype.Text        `db:"sender_host"`
}

// PgReplicationSlot contains per-slot state from pg_replication_slots.
//
// Version-gated:
//   - WalStatus, SafeWalSizeBytes: PG 13+
//   - Conflicting: PG 16+
//
// RetainedWalBytes is computed primary-side against pg_current_wal_lsn();
// NULL on standbys (CASE WHEN pg_is_in_recovery()). Physical and logical
// slots share the view — SlotType differentiates them.
type PgReplicationSlot struct {
	Database          pgtype.Text   `db:"database"`
	SlotName          pgtype.Text   `db:"slot_name"`
	Plugin            pgtype.Text   `db:"plugin"`
	SlotType          pgtype.Text   `db:"slot_type"`
	DatName           pgtype.Text   `db:"datname"`
	Active            pgtype.Bool   `db:"active"`
	Temporary         pgtype.Bool   `db:"temporary"`
	RestartLsnBytes   pgtype.Float8 `db:"restart_lsn_bytes"`
	ConfirmedFlushLsn pgtype.Float8 `db:"confirmed_flush_lsn_bytes"`
	RetainedWalBytes  pgtype.Float8 `db:"retained_wal_bytes"`
	WalStatus         pgtype.Text   `db:"wal_status"`          // PG 13+
	SafeWalSizeBytes  pgtype.Int8   `db:"safe_wal_size_bytes"` // PG 13+
	Conflicting       pgtype.Bool   `db:"conflicting"`         // PG 16+
}

// PgDatabaseSize contains on-disk size per database (via pg_database_size()).
type PgDatabaseSize struct {
	Database pgtype.Text `db:"database"`
	DatName  pgtype.Text `db:"datname"`
	Bytes    pgtype.Int8 `db:"bytes"`
}

// PgLock contains information on locks held.
type PgLock struct {
	Database pgtype.Text `db:"database"`
	DatName  pgtype.Text `db:"datname"`
	Mode     pgtype.Text `db:"mode"`
	Count    pgtype.Int8 `db:"count"`
}

// PgStatActivity contains information on tx state.
type PgStatActivity struct {
	Database      pgtype.Text   `db:"database"`
	DatName       pgtype.Text   `db:"datname"`
	State         pgtype.Text   `db:"state"`
	Count         pgtype.Int8   `db:"count"`
	MaxTxDuration pgtype.Float8 `db:"max_tx_duration"`
}

// PgStatUserTable contains information on user tables.
//
// Some columns are version-gated — the SQL that populates this struct only
// selects them on Postgres versions that expose them. Version-gated fields
// remain zero-valued (pgtype.Valid == false) on older servers, and the
// collector skips them. Mapping:
//
//   - NInsSinceVacuum:  PG 13+
//   - LastSeqScan, LastIdxScan:  PG 16+
//   - NTupNewpageUpdate:  PG 17+
type PgStatUserTable struct {
	Database          pgtype.Text        `db:"database"`
	SchemaName        pgtype.Text        `db:"schemaname"`
	RelName           pgtype.Text        `db:"relname"`
	SeqScan           pgtype.Int8        `db:"seq_scan"`
	LastSeqScan       pgtype.Timestamptz `db:"last_seq_scan"` // PG 16+
	SeqTupRead        pgtype.Int8        `db:"seq_tup_read"`
	IndexScan         pgtype.Int8        `db:"idx_scan"`
	LastIdxScan       pgtype.Timestamptz `db:"last_idx_scan"` // PG 16+
	IndexTupFetch     pgtype.Int8        `db:"idx_tup_fetch"`
	NTupInsert        pgtype.Int8        `db:"n_tup_ins"`
	NTupUpdate        pgtype.Int8        `db:"n_tup_upd"`
	NTupDelete        pgtype.Int8        `db:"n_tup_del"`
	NTupHotUpdate     pgtype.Int8        `db:"n_tup_hot_upd"`
	NTupNewpageUpdate pgtype.Int8        `db:"n_tup_newpage_upd"` // PG 17+
	NLiveTup          pgtype.Int8        `db:"n_live_tup"`
	NDeadTup          pgtype.Int8        `db:"n_dead_tup"`
	NModSinceAnalyze  pgtype.Int8        `db:"n_mod_since_analyze"`
	NInsSinceVacuum   pgtype.Int8        `db:"n_ins_since_vacuum"` // PG 13+
	LastVacuum        pgtype.Timestamptz `db:"last_vacuum"`
	LastAutoVacuum    pgtype.Timestamptz `db:"last_autovacuum"`
	LastAnalyze       pgtype.Timestamptz `db:"last_analyze"`
	LastAutoAnalyze   pgtype.Timestamptz `db:"last_autoanalyze"`
	VacuumCount       pgtype.Int8        `db:"vacuum_count"`
	AutoVacuumCount   pgtype.Int8        `db:"autovacuum_count"`
	AnalyzeCount      pgtype.Int8        `db:"analyze_count"`
	AutoAnalyzeCount  pgtype.Int8        `db:"autoanalyze_count"`
}

// PgStatIOUserTable contains I/O information on user tables.
type PgStatIOUserTable struct {
	Database      pgtype.Text `db:"database"`
	SchemaName    pgtype.Text `db:"schemaname"`
	RelName       pgtype.Text `db:"relname"`
	HeapBlksRead  pgtype.Int8 `db:"heap_blks_read"`
	HeapBlksHit   pgtype.Int8 `db:"heap_blks_hit"`
	IndexBlksRead pgtype.Int8 `db:"idx_blks_read"`
	IndexBlksHit  pgtype.Int8 `db:"idx_blks_hit"`
	ToastBlksRead pgtype.Int8 `db:"toast_blks_read"`
	ToastBlksHit  pgtype.Int8 `db:"toast_blks_hit"`
	TidxBlksRead  pgtype.Int8 `db:"tidx_blks_read"`
	TidxBlksHit   pgtype.Int8 `db:"tidx_blks_hit"`
}

// PgStatUserIndexes contains information on user indexes.
type PgStatUserIndex struct {
	Database      pgtype.Text `db:"database"`
	SchemaName    pgtype.Text `db:"schemaname"`
	RelName       pgtype.Text `db:"relname"`
	IndexRelName  pgtype.Text `db:"indexrelname"`
	IndexScan     pgtype.Int8 `db:"idx_scan"`
	IndexTupRead  pgtype.Int8 `db:"idx_tup_read"`
	IndexTupFetch pgtype.Int8 `db:"idx_tup_fetch"`
}

// PgStatIOUserIndex contains I/O information on user indexes.
type PgStatIOUserIndex struct {
	Database      pgtype.Text `db:"database"`
	SchemaName    pgtype.Text `db:"schemaname"`
	RelName       pgtype.Text `db:"relname"`
	IndexRelName  pgtype.Text `db:"indexrelname"`
	IndexBlksRead pgtype.Int8 `db:"idx_blks_read"`
	IndexBlksHit  pgtype.Int8 `db:"idx_blks_hit"`
}

// PgStatStatement contains information on statements.
type PgStatStatement struct {
	Database            pgtype.Text   `db:"database"`
	RolName             pgtype.Text   `db:"rolname"`
	DatName             pgtype.Text   `db:"datname"`
	QueryID             pgtype.Int8   `db:"queryid"`
	Query               pgtype.Text   `db:"query"`
	Calls               pgtype.Int8   `db:"calls"`
	TotalTimeSeconds    pgtype.Float8 `db:"total_time_seconds"`
	MinTimeSeconds      pgtype.Float8 `db:"min_time_seconds"`
	MaxTimeSeconds      pgtype.Float8 `db:"max_time_seconds"`
	MeanTimeSeconds     pgtype.Float8 `db:"mean_time_seconds"`
	StdDevTimeSeconds   pgtype.Float8 `db:"stddev_time_seconds"`
	Rows                pgtype.Int8   `db:"rows"`
	SharedBlksHit       pgtype.Int8   `db:"shared_blks_hit"`
	SharedBlksRead      pgtype.Int8   `db:"shared_blks_read"`
	SharedBlksDirtied   pgtype.Int8   `db:"shared_blks_dirtied"`
	SharedBlksWritten   pgtype.Int8   `db:"shared_blks_written"`
	LocalBlksHit        pgtype.Int8   `db:"local_blks_hit"`
	LocalBlksRead       pgtype.Int8   `db:"local_blks_read"`
	LocalBlksDirtied    pgtype.Int8   `db:"local_blks_dirtied"`
	LocalBlksWritten    pgtype.Int8   `db:"local_blks_written"`
	TempBlksRead        pgtype.Int8   `db:"temp_blks_read"`
	TempBlksWritten     pgtype.Int8   `db:"temp_blks_written"`
	BlkReadTimeSeconds  pgtype.Float8 `db:"blk_read_time_seconds"`
	BlkWriteTimeSeconds pgtype.Float8 `db:"blk_write_time_seconds"`
}
