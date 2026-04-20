// Package model defines the row structs that scany scans
// pg_stat_* / pg_statio_* / pg_* query results into.
//
// Every column uses a [pgtype.*] nullable wrapper — collectors check
// .Valid before emitting a metric so NULL counters (e.g. partitioned
// tables on PG 17, pre-reset statistics views, standby-only columns)
// are skipped instead of becoming a stream of zeros.
package model
