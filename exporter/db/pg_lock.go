package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

const sqlSelectPgLock = `
SELECT 
    current_database() as database,
    pg_database.datname,
    tmp.mode,
    COALESCE(count,0) as count
FROM
(
  VALUES ('accesssharelock'),
         ('rowsharelock'),
         ('rowexclusivelock'),
         ('shareupdateexclusivelock'),
         ('sharelock'),
         ('sharerowexclusivelock'),
         ('exclusivelock'),
         ('accessexclusivelock'),
 ('sireadlock')
) AS tmp(mode) CROSS JOIN pg_database
LEFT JOIN
  (SELECT database, lower(mode) AS mode,count(*) AS count
  FROM pg_locks WHERE database IS NOT NULL
  GROUP BY database, lower(mode)
) AS tmp2
ON tmp.mode=tmp2.mode and pg_database.oid = tmp2.database ORDER BY 1`

// SelectPgLocks selects stats on locks held.
func (db *Client) SelectPgLocks(ctx context.Context) ([]*model.PgLock, error) {
	pgLocks := []*model.PgLock{}
	if err := db.Select(ctx, &pgLocks, sqlSelectPgLock); err != nil {
		return nil, err
	}
	return pgLocks, nil
}
