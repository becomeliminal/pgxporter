package main

import (
	"context"

	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
)

func main() {
	exporter := exporter.MustNew(context.Background(), exporter.Opts{DBOpts: []db.Opts{{}}})
	exporter.Register()
}
