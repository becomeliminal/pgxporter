package db

import (
	"context"
	"time"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

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

	for i := 0; i <= opts.MaxConnectionRetries || opts.MaxConnectionRetries == -1; i++ {
		pool, err = pgxpool.ConnectConfig(ctx, poolConfig)
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
		return client, nil
	}
	return nil, err
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
