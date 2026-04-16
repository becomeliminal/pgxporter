package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

const sqlSelectPgStatActivity = `
SELECT
  current_database() as database,
  pg_database.datname,
  tmp.state,
  COALESCE(count,0) as count,
  COALESCE(max_tx_duration,0) as max_tx_duration
FROM
(
  VALUES ('active'),
   ('idle'),
   ('idle in transaction'),
   ('idle in transaction (aborted)'),
   ('fastpath function call'),
   ('disabled')
) AS tmp(state) CROSS JOIN pg_database
LEFT JOIN
(
SELECT
  datname,
  state,
  count(*) AS count,
  MAX(EXTRACT(EPOCH FROM now() - xact_start))::float AS max_tx_duration
FROM pg_stat_activity GROUP BY datname,state) AS tmp2
ON tmp.state = tmp2.state AND pg_database.datname = tmp2.datname`

// SelectPgStatActivity selects stats on user tables.
func (db *Client) SelectPgStatActivity(ctx context.Context) ([]*model.PgStatActivity, error) {
	pgStatActivities := []*model.PgStatActivity{}
	if err := db.Select(ctx, &pgStatActivities, sqlSelectPgStatActivity); err != nil {
		return nil, err
	}
	return pgStatActivities, nil
}
