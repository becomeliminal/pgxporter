package db

import (
	"time"
)

// Opts specify the configuration for a postgres client.
type Opts struct {
	//
	Host            string `long:"postgres_host"     env:"POSTGRES_HOST"     default:"localhost" description:"Postgres host"`
	Port            int    `long:"postgres_port"     env:"POSTGRES_PORT"     default:"5432"     description:"Postgres port"`
	User            string `long:"postgres_user"     env:"POSTGRES_USER"     default:"postgres" description:"Postgres username"`
	Password        string `long:"postgres_password" env:"POSTGRES_PASSWORD" default:"postgres" description:"Postgres password"`
	Database        string `long:"postgres_database" env:"POSTGRES_DATABASE" default:"postgres" description:"Postgres database"`
	AuthMechanism   string `long:"auth_mechanism" env:"AUTH_MECHANISM" description:"The mechanism to use when authenticating with the DB" choice:"password" choice:"client_certificates" default:"password"`
	ApplicationName string `long:"application_name" env:"APP_NAME" required:"true"`
	// Connection parameters.
	ConnectTimeout       time.Duration `long:"connect_timeout" env:"CONNECT_TIMEOUT" default:"10s" description:"Postgres connection timeout"`
	MaxConnectionRetries int           `long:"max_retries" env:"MAX_RETRIES" default:"6" description:"Max number of retry attempts when forming a connection to the database before giving up. (0 is no retries, -1 is infinite retries or, if possible, until the context times out)."`
	// Client connection optional parameters.
	DefaultIsolationLevel           string        `long:"default_isolation_level" env:"DEFAULT_ISOLATION_LEVEL" default:"REPEATABLE_READ" description:"default isolation level for DB transactions" choice:"READ_COMMITTED" choice:"REPEATABLE_READ" choice:"SERIALIZABLE"`
	StatementTimeout                time.Duration `long:"statement_timeout" env:"STATEMENT_TIMEOUT" default:"5s" description:"Abort any statement that takes more than the specified number of milliseconds, starting from the time the command arrives at the server from the client. A value of zero (the default) turns this off."`
	LockTimeout                     time.Duration `long:"lock_timeout" env:"LOCK_TIMEOUT" default:"0ms" description:"Abort any statement that waits longer than the specified number of milliseconds while attempting to acquire a lock on a table, index, row, or other database object. The time limit applies separately to each lock acquisition attempt. The limit applies both to explicit locking requests (such as LOCK TABLE, or SELECT FOR UPDATE without NOWAIT) and to implicitly-acquired locks. A value of zero (the default) turns this off."`
	IdleInTransactionSessionTimeout time.Duration `long:"idle_in_transaction_session_timeout" env:"IDLE_IN_TRANSACTION_SESSION_TIMEOUT" default:"5s" description:"Terminate any session with an open transaction that has been idle for longer than the specified duration in milliseconds. This allows any locks held by that session to be released and the connection slot to be reused; it also allows tuples visible only to this transaction to be vacuumed. A value of zero (the default) turns this off."`
	// Transaction options.
	TotalTransactionTimeout      time.Duration `long:"total_transaction_timeout" env:"TOTAL_TRANSACTION_TIMEOUT" default:"5s" description:"The total time spent waiting for a transaction to finish, including retries, before cancelling it client side."`
	InitialTransactionRetryDelay time.Duration `long:"initial_transaction_retry_delay" env:"INITIAL_TRANSACTION_RETRY_DELAY" default:"50ms" description:"The initial duration of time to wait before retrying a transaction attempt"`
	BaseTransactionRetryDelay    time.Duration `long:"base_transaction_retry_delay" env:"BASE_TRANSACTION_RETRY_DELAY" default:"50ms" description:"The duration of time to wait before retrying a transaction attempt"`
	MaxTransactionAttempts       int           `long:"max_transaction_attempts" env:"MAX_TRANSACTION_ATTEMPTS" default:"-1" description:"The maximum number of attempts at executing a transaction (-1 is infinite or until the context expires)."`
	ReadOnly                     bool          `long:"read_only" env:"READ_ONLY" description:"Determines whether transactions are read-only and can be routed to read-replicas."`
	// pgxpool.ConnConfig
	PoolMaxConns          int           `long:"pool_max_conns" env:"MAX_CONNS" default:"10" description:"Max open connections to the database"`
	PoolMinConns          int           `long:"pool_min_conns" env:"MIN_CONNS" default:"2" description:"Min open connections to the database"`
	PoolMaxConnLifetime   time.Duration `long:"pool_max_conn_lifetime" env:"MAX_CONN_LIFETIME" default:"1h" description:"Max connection lifetime, after which, connections will be lazily closed"`
	PoolMaxConnIdleTime   time.Duration `long:"pool_max_conn_idle_time" env:"MAX_CONN_IDLE_TIME" default:"30m" description:"Max connection idle time, after which, connections will be lazily closed"`
	PoolHealthCheckPeriod time.Duration `long:"pool_health_check_period" env:"HEALTH_CHECK_PERIOD" default:"1m" description:"Health check period is the duration between checks of the health of idle connections"`
	// pgx.ConnConfig
	StatementCacheCapacity int    `long:"statement_cache_capacity" env:"STATEMENT_CACHE_CAPACITY" default:"512" description:"The maximum number of prepared statements in the automatic statement cache. Set to 0 disable automatic statement caching"`
	StatementCacheMode     string `long:"statement_cache_mode" env:"STATEMENT_CACHE_MODE" default:"prepare" description:"Prepare will create prepared statements on the PostgreSQL server. Describe will use the anonymous prepared statement to describe a statement without creating a statement on the server. Describe is primarily useful when the environment does not allow prepared statements such as when running a connection poller like PgBouncer or DeadPool" choice:"prepare" choice:"describe"`

	// AuthProvider, if non-nil, mints the password for every new pool
	// connection via pgx's BeforeConnect hook. Replaces Password. See
	// the AuthProvider godoc for cloud-IAM use cases.
	AuthProvider AuthProvider `no-flag:"true"`
}
