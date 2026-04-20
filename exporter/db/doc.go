// Package db wraps pgx/v5's pgxpool with the thin conveniences the
// exporter needs: [New] applies [Opts.StatementCacheMode] and the
// [AuthProvider] BeforeConnect hook, probes the server version once
// (cached on [Client.ServerVersionNum]), and surfaces [Client.Select]
// as the scany-scanned equivalent of pgxpool.Query for collector code.
//
// Every SQL builder in this package (SelectPgStatActivity,
// SelectPgStatDatabase, etc.) version-gates its column set via
// [Client.AtLeast] so a single binary runs clean against PG 13–18.
//
// For cloud-IAM auth (AWS RDS, GCP CloudSQL, Azure), set [Opts.AuthProvider]
// to a provider that implements [AuthProvider.Password]; pgx will call it
// on every new pool connection. See the sub-package auth/awsrds for the
// canonical example.
package db
