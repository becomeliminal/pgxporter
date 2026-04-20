// Command pgxporter is a minimal reference binary wrapping the exporter
// library. It serves /metrics over HTTP for one PostgreSQL instance
// configured via a libpq-style DSN environment variable.
//
// This is intentionally minimal — a production deployment typically writes
// its own main with richer flag libraries, service-discovery integration,
// TLS listener config (via prometheus/exporter-toolkit), etc. See the
// README's example deployments section. The primary purpose of this
// binary today is to back the head-to-head benchmark so pgxporter can be
// compared to postgres_exporter apples-to-apples — both as subprocesses,
// observed from the same client vantage point.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/becomeliminal/pgxporter/exporter"
	"github.com/becomeliminal/pgxporter/exporter/db"
)

func main() {
	listen := flag.String("web.listen-address", ":9187", "Address to listen on for /metrics")
	metricsPath := flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics")
	timeout := flag.Duration("collection-timeout", time.Minute, "Per-scrape deadline")
	logLevel := flag.String("log.level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	slog.SetDefault(newLogger(*logLevel))

	dsn := os.Getenv("DATA_SOURCE_NAME")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATA_SOURCE_NAME env var is required (libpq DSN, URI or key=value form)")
		os.Exit(2)
	}

	dbOpts, err := parseDSN(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parsing DATA_SOURCE_NAME: %v\n", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	exp, err := exporter.New(ctx, exporter.Opts{
		DBOpts:            []db.Opts{dbOpts},
		CollectionTimeout: *timeout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting exporter: %v\n", err)
		os.Exit(1)
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(exp); err != nil {
		fmt.Fprintf(os.Stderr, "registering collectors: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{Timeout: *timeout}))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `<html><body><h1>pgxporter</h1><a href="%s">/metrics</a></body></html>`, *metricsPath)
	})

	srv := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", *listen, "path", *metricsPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "listener: %v\n", err)
		os.Exit(1)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", "err", err)
	}
	if err := exp.Shutdown(shutdownCtx); err != nil {
		slog.Error("exporter shutdown", "err", err)
	}
}

// parseDSN converts a libpq-style DSN (URI or key=value) into a db.Opts.
// Uses pgx's own config parser so every format libpq accepts works here,
// including sslmode, passfile, and per-host-port lists.
func parseDSN(dsn string) (db.Opts, error) {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return db.Opts{}, err
	}
	return db.Opts{
		Host:                  cfg.Host,
		Port:                  int(cfg.Port),
		User:                  cfg.User,
		Password:              cfg.Password,
		Database:              cfg.Database,
		ApplicationName:       applicationNameOr(cfg, "pgxporter"),
		ConnectTimeout:        10 * time.Second,
		PoolMaxConns:          4,
		PoolMinConns:          1,
		PoolMaxConnLifetime:   time.Hour,
		PoolMaxConnIdleTime:   30 * time.Minute,
		PoolHealthCheckPeriod: time.Minute,
		StatementCacheMode:    "prepare",
	}, nil
}

func applicationNameOr(cfg *pgconn.Config, fallback string) string {
	if v, ok := cfg.RuntimeParams["application_name"]; ok && v != "" {
		return v
	}
	return fallback
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
