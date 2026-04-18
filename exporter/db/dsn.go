package db

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// DSN constructs a postgres-compatible connection URI.
func DSN(opts Opts) string {
	dsn := fmt.Sprintf("postgresql://%s", url.QueryEscape(opts.User))
	opts = configureAuthParams(opts)
	if opts.Password != "" {
		dsn += fmt.Sprintf(":%s", url.QueryEscape(opts.Password))
	}
	dsn += fmt.Sprintf("@%s:%v/%s", opts.Host, opts.Port, url.QueryEscape(opts.Database))

	// Application name here.
	return addParametersToDSN(dsn, opts, false)
}

func configureAuthParams(opts Opts) Opts {
	if opts.AuthMechanism == "" {
		opts.AuthMechanism = opts.Password
	}
	switch opts.AuthMechanism {
	case opts.Password:
		// If password is not provided, grab from secrets dir.
	default:
	}
	return opts

}

func addParametersToDSN(dsn string, opts Opts, postgresCompatible bool) string {
	var params parameters
	// params = append(params, "sslmode", opts.ConnectionSSLMode())
	// TODO: SSL STUFF
	params = appendParam(params, "application_name", opts.ApplicationName)
	if !postgresCompatible {
		params = appendParam(params, "pool_max_conns", strconv.Itoa(opts.PoolMaxConns))
		params = appendParam(params, "pool_min_conns", strconv.Itoa(opts.PoolMinConns))
		params = appendParam(params, "pool_max_conn_lifetime", fmt.Sprintf("%.0fs", opts.PoolMaxConnLifetime.Seconds()))
		params = appendParam(params, "pool_max_conn_idle_time", fmt.Sprintf("%.0fs", opts.PoolMaxConnIdleTime.Seconds()))
		params = appendParam(params, "pool_health_check_period", fmt.Sprintf("%.0fs", opts.PoolHealthCheckPeriod.Seconds()))
		// statement_cache_capacity / statement_cache_mode were pgx v4 DSN params.
		// Under pgx v5, StatementCacheMode is applied via ConnConfig.DefaultQueryExecMode
		// in New(); StatementCacheCapacity is a no-op (v5 has no LRU cap).
	}

	var optionParams optionParameters
	optionParams = appendOptionsParam(optionParams, "default_transaction_isolation", string(defaultIsolationLevel(opts.DefaultIsolationLevel)))
	optionParams = appendOptionsParam(optionParams, "timezone", "UTC")
	optionParams = appendOptionsParam(optionParams, "statement_timeout", fmt.Sprintf("%d", opts.StatementTimeout.Milliseconds()))
	optionParams = appendOptionsParam(optionParams, "lock_timeout", fmt.Sprintf("%d", opts.LockTimeout.Milliseconds()))
	optionParams = appendOptionsParam(optionParams, "idle_in_transaction_session_timeout", fmt.Sprintf("%d", opts.IdleInTransactionSessionTimeout.Milliseconds()))

	params = appendOptionsParams(params, optionParams)
	if len(params) > 0 {
		dsn = strings.Join([]string{dsn, params.String()}, "?")
	}
	return dsn
}

type parameters []string

func (p *parameters) String() string {
	return strings.Join([]string(*p), "&")
}

func appendParam(p parameters, key, value string) parameters {
	if len(key) > 0 && len(value) > 0 {
		return append(p, fmt.Sprintf("%s=%s", url.QueryEscape(key), url.QueryEscape(value)))
	}
	return p
}

type optionParameters []string

func (op *optionParameters) String() string {
	return strings.Join([]string(*op), "%20")
}

func appendOptionsParam(op optionParameters, key, value string) optionParameters {
	if len(key) > 0 && len(value) > 0 {
		return append(op, strings.Join([]string{"-c", formatOptionsParam(key), url.QueryEscape("="), formatOptionsParam(value)}, ""))
	}
	return op
}

func appendOptionsParams(p parameters, op optionParameters) parameters {
	if len(op) > 0 {
		return append(p, strings.Join([]string{"options=", op.String()}, ""))
	}
	return p
}

func formatOptionsParam(str string) string {
	fields := strings.Fields(str)
	for i := range fields {
		fields[i] = url.QueryEscape(fields[i])
	}
	return strings.Join([]string(fields), "\\%20")
}
