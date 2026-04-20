package db

import (
	"context"
)

// AuthProvider produces the password used for a connection attempt.
//
// The flagship pgx-native advantage: because pgx v5 exposes a
// BeforeConnect hook on pgxpool.Config, we can mint a fresh short-lived
// password per connection attempt without any DSN-rewriting wrapper.
// prometheus-community/postgres_exporter cannot do this cleanly because
// lib/pq has no equivalent hook — users have to rebuild the DSN out-of-
// band and feed it in via --probe.target or similar workarounds.
//
// Typical implementations:
//
//   - AWS RDS IAM: signs a v4 auth token scoped to (host, port, user)
//     — no network call, pure local crypto. See exporter/db/auth/awsrds.
//   - GCP Cloud SQL IAM: calls the IAM token service to mint a short-
//     lived OAuth token, passes it as the PG password.
//   - Azure Database for PostgreSQL: fetches a Microsoft Entra token
//     via DefaultAzureCredential and uses it as the password.
//   - A local Vault / static-rotation provider: reads a credential from
//     a file/KV every N minutes.
//
// The Opts.Password field is the fallback when AuthProvider is nil.
// Callers MUST ensure ctx cancellation is honoured — BeforeConnect is
// called on every new pool connection and a misbehaving provider will
// block pool expansion indefinitely.
type AuthProvider interface {
	// Password returns the password to use for a connection to
	// (host, port, user). Implementations should return a fresh token on
	// every call — pgx caches connections, not passwords, so expiry is
	// bounded by PoolMaxConnLifetime.
	Password(ctx context.Context, host string, port int, user string) (string, error)
}

// StaticAuthProvider returns a fixed password. Equivalent to setting
// Opts.Password, but exposed as an AuthProvider so tests can exercise
// the BeforeConnect hook without needing a cloud SDK.
type StaticAuthProvider struct {
	Pwd string
}

// Password implements AuthProvider.
func (s StaticAuthProvider) Password(_ context.Context, _ string, _ int, _ string) (string, error) {
	return s.Pwd, nil
}
