// Package awsrds provides an AuthProvider that mints AWS RDS / Aurora
// IAM auth tokens for pgxporter's pgx v5 BeforeConnect hook.
//
// Token minting is pure local SigV4 signing — no network round-trip per
// connection. Tokens are valid for 15 minutes; pgxpool hands them out on
// every new connection, so short-lived connections get fresh tokens.
// Set Opts.PoolMaxConnLifetime ≤ 14 minutes to guarantee every in-use
// connection is backed by a still-valid token.
//
// Usage:
//
//	ctx := context.Background()
//	provider, err := awsrds.NewDefault(ctx, "us-east-1")
//	if err != nil { return err }
//	opts := db.Opts{
//	    Host:         "mydb.cluster-xyz.us-east-1.rds.amazonaws.com",
//	    Port:         5432,
//	    User:         "app_iam_user",
//	    AuthProvider: provider,
//	    // Rotate connections ahead of the 15-minute token expiry.
//	    PoolMaxConnLifetime: 14 * time.Minute,
//	}
//	client, err := db.New(ctx, opts)
package awsrds

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	rdsauth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
)

// Provider implements db.AuthProvider by calling rdsauth.BuildAuthToken
// on every Password() invocation.
type Provider struct {
	// Region is the AWS region the RDS/Aurora instance lives in, e.g.
	// "us-east-1". Required.
	Region string
	// Credentials is the AWS credentials provider used for signing.
	// Typically obtained via config.LoadDefaultConfig, which chains
	// env vars → shared config → IRSA → IMDS.
	Credentials aws.CredentialsProvider
}

// NewDefault loads the AWS default config chain (env → shared config →
// IRSA → IMDS) and returns a Provider scoped to the given region.
func NewDefault(ctx context.Context, region string) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS default config: %w", err)
	}
	return &Provider{Region: region, Credentials: cfg.Credentials}, nil
}

// Password returns a freshly signed RDS IAM auth token for (host, port, user).
// The returned string is used as the PG password on the wire.
func (p *Provider) Password(ctx context.Context, host string, port int, user string) (string, error) {
	if p.Region == "" {
		return "", fmt.Errorf("awsrds: Region is required")
	}
	if p.Credentials == nil {
		return "", fmt.Errorf("awsrds: Credentials is required")
	}
	endpoint := fmt.Sprintf("%s:%d", host, port)
	return rdsauth.BuildAuthToken(ctx, endpoint, p.Region, user, p.Credentials)
}
