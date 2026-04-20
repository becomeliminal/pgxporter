package db

import (
	"context"
	"fmt"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/becomeliminal/pgxporter/exporter/logging"
)

var log = logging.NewLogger()

const (
	connectRetryWait = 2 * time.Second
)

// Client to PostgreSQL server.
type Client struct {
	opts      Opts
	pool      *pgxpool.Pool
	txOptions pgx.TxOptions

	// ServerVersionNum is the Postgres server's numeric version (e.g. 170006
	// for PG 17.6) as returned by SHOW server_version_num. Cached once at
	// [New] time. Zero if the probe failed (shouldn't happen for a healthy
	// connection; collectors that need version gating should treat zero as
	// "unknown — fall back to the most conservative SQL").
	ServerVersionNum int
}

// AtLeast reports whether the connected Postgres server is at least
// the given major.minor version. Returns false if version detection
// failed at startup.
//
// Example:
//
//	if client.AtLeast(17, 0) {
//	    // include columns added in PG 17
//	}
func (c *Client) AtLeast(major, minor int) bool {
	if c.ServerVersionNum == 0 {
		return false
	}
	// Postgres encodes server_version_num as major*10000 + minor
	// (e.g. 16.4 → 160004). Match that exactly.
	return c.ServerVersionNum >= major*10000+minor
}

// New instantiates and returns a new DB.
func New(ctx context.Context, opts Opts) (*Client, error) {
	var pool *pgxpool.Pool
	var err error
	dsn := DSN(opts)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	switch opts.StatementCacheMode {
	case "prepare":
		poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	case "describe":
		poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe
	}

	// If an AuthProvider is configured, wire it to pgx's BeforeConnect
	// hook so every new pool connection gets a freshly-minted password.
	// This is how cloud-IAM auth (RDS, CloudSQL, Azure) plugs in without
	// DSN-rewriting hacks.
	if opts.AuthProvider != nil {
		provider := opts.AuthProvider
		poolConfig.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
			pwd, err := provider.Password(ctx, cc.Host, int(cc.Port), cc.User)
			if err != nil {
				return fmt.Errorf("auth provider: %w", err)
			}
			cc.Password = pwd
			return nil
		}
	}

	for i := 0; i <= opts.MaxConnectionRetries || opts.MaxConnectionRetries == -1; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			if err == ctx.Err() {
				return nil, err
			}
			if i < opts.MaxConnectionRetries && opts.MaxConnectionRetries != 0 {
				time.Sleep(connectRetryWait)
			}
			continue
		}
		client := &Client{
			opts: opts,
			pool: pool,
		}
		client.setTxOptions(opts)
		if err := client.probeServerVersion(ctx); err != nil {
			// Non-fatal: a collector that version-gates will treat zero as
			// "unknown". Log and keep going.
			log.Warn("could not detect Postgres server version", "err", err)
		}
		return client, nil
	}
	return nil, err
}

// probeServerVersion reads server_version_num once and caches the result.
//
// Uses SELECT … ::int (rather than SHOW) so pgx sees an int4 column directly
// instead of text, avoiding a client-side parse.
func (c *Client) probeServerVersion(ctx context.Context) error {
	var v int
	if err := c.pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&v); err != nil {
		return err
	}
	c.ServerVersionNum = v
	log.Info("connected to Postgres", "major", v/10000, "minor", v%10000, "server_version_num", v)
	return nil
}

func (c *Client) setTxOptions(opts Opts) {
	iso := defaultIsolationLevel(opts.DefaultIsolationLevel)
	c.txOptions = pgx.TxOptions{
		IsoLevel:   iso,
		AccessMode: pgx.ReadWrite,
	}
	if opts.ReadOnly {
		c.txOptions.AccessMode = pgx.ReadOnly
	}
}

func defaultIsolationLevel(isoLevel string) pgx.TxIsoLevel {
	switch isoLevel {
	case "READ_COMMITTED":
		return pgx.ReadCommitted
	case "SERIALIZABLE":
		return pgx.Serializable
	}
	return pgx.RepeatableRead
}

// CheckConnection acquires a connection from the pool and executes an empty sql statement over it.
func (c *Client) CheckConnection(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// Select executes a statement that fetches rows in a transaction.
func (c *Client) Select(ctx context.Context, dest interface{}, sql string, args ...interface{}) error {
	rows, err := c.pool.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	return pgxscan.ScanAll(dest, rows)
}

// Query exposes the underlying pgxpool.Query for callers that want raw
// pgx.Rows rather than scany-scanned structs. Used by the declarative
// SpecCollector, which reads columns by name at runtime.
func (c *Client) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return c.pool.Query(ctx, sql, args...)
}
