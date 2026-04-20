package db

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter/db/model"
)

// SelectPgSettings returns the numeric-typed subset of pg_settings
// (vartype IN ('bool','integer','real')). String and enum settings are
// filtered out — they'd need info-metric handling which we defer. Bool
// settings are mapped to 0/1 in SQL so the Go side only sees a Float8.
func (db *Client) SelectPgSettings(ctx context.Context) ([]*model.PgSetting, error) {
	rows := []*model.PgSetting{}
	const sql = `SELECT
		current_database() AS database,
		name,
		coalesce(unit, '') AS unit,
		vartype,
		CASE
			WHEN vartype = 'bool' THEN CASE WHEN setting = 'on' THEN 1 ELSE 0 END
			WHEN setting ~ '^-?[0-9]+(\.[0-9]+)?$' THEN setting::float8
			ELSE 0
		END AS value
	FROM pg_settings
	WHERE vartype IN ('bool', 'integer', 'real')`
	if err := db.Select(ctx, &rows, sql); err != nil {
		return nil, err
	}
	return rows, nil
}
