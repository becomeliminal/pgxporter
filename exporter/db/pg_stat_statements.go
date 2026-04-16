package db

import (
	"context"
	"time"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

const sqlSelectPgStatStatements = `
SELECT 
    current_database() as database,
    t2.rolname, 
    t3.datname, 
    queryid,
    concat(left(query, 200), '__@',queryid::text, '__', length(query)) as query,
    calls, 
    total_time / 1000 as total_time_seconds, 
    min_time / 1000 as min_time_seconds, 
    max_time / 1000 as max_time_seconds, 
    mean_time / 1000 as mean_time_seconds, 
    stddev_time / 1000 as stddev_time_seconds, 
    rows, 
    shared_blks_hit, 
    shared_blks_read, 
    shared_blks_dirtied, 
    shared_blks_written, 
    local_blks_hit, 
    local_blks_read, 
    local_blks_dirtied, 
    local_blks_written, 
    temp_blks_read, 
    temp_blks_written, 
    blk_read_time / 1000 as blk_read_time_seconds, 
    blk_write_time / 1000 as blk_write_time_seconds 
FROM pg_stat_statements t1 
JOIN pg_roles t2 ON (t1.userid=t2.oid) 
JOIN pg_database t3 ON (t1.dbid=t3.oid) 
WHERE t2.rolname != 'rdsadmin'`

// SelectPgStatStatements selects stats on user tables.
func (db *Client) SelectPgStatStatements(ctx context.Context) ([]*model.PgStatStatement, error) {
	start := time.Now()
	pgStatStatements := []*model.PgStatStatement{}
	if err := db.Select(ctx, &pgStatStatements, sqlSelectPgStatStatements); err != nil {
		return nil, err
	}
	log.Infof("%s select statements took %dms", db.opts.Database, time.Now().Sub(start).Milliseconds())
	return pgStatStatements, nil
}
