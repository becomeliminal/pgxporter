package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgSettings returns every numeric or bool setting from
// pg_settings. Text and enum settings are skipped — they don't have a
// natural float64 representation. Bool settings convert on→1, off→0.
//
// Cardinality is bounded by PG's built-in GUC count (~300) plus any
// loaded extensions. Enabling this collector on an instance with many
// extensions can push the series count a bit but is nowhere near the
// cardinality hazards of per-table views.
func (db *Client) SelectPgSettings(ctx context.Context) ([]*model.PgSetting, error) {
	rows := []*model.PgSetting{}
	const sql = `SELECT
		current_database() AS database,
		name,
		COALESCE(unit, '') AS unit,
		vartype,
		CASE
			WHEN vartype = 'bool' THEN CASE WHEN setting = 'on' THEN 1.0::float8 ELSE 0.0::float8 END
			ELSE setting::float8
		END AS value
	FROM pg_settings
	WHERE vartype IN ('bool', 'integer', 'real')`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
