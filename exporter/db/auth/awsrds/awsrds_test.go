package awsrds

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func TestProvider_Password_GeneratesSignedToken(t *testing.T) {
	// Fake creds — enough to exercise SigV4 signing without hitting AWS.
	p := &Provider{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAFAKE", "secretfake", ""),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token, err := p.Password(ctx, "mydb.cluster-xyz.us-east-1.rds.amazonaws.com", 5432, "app")
	if err != nil {
		t.Fatalf("Password: %v", err)
	}
	// An RDS IAM token is a URL-encoded presigned request; it always
	// contains the action, the access key, and an X-Amz-Signature.
	for _, want := range []string{
		"Action=connect",
		"DBUser=app",
		"X-Amz-Signature=",
		"AKIAFAKE",
	} {
		if !strings.Contains(token, want) {
			t.Errorf("token missing %q:\n%s", want, token)
		}
	}
}

func TestProvider_Password_RequiresRegionAndCreds(t *testing.T) {
	ctx := context.Background()
	if _, err := (&Provider{}).Password(ctx, "h", 5432, "u"); err == nil {
		t.Errorf("empty provider should error")
	}
	if _, err := (&Provider{Region: "us-east-1"}).Password(ctx, "h", 5432, "u"); err == nil {
		t.Errorf("missing credentials should error")
	}
}

// Compile-time assertion that *Provider satisfies db.AuthProvider.
// Kept in this package to avoid an import cycle.
var _ interface {
	Password(ctx context.Context, host string, port int, user string) (string, error)
} = (*Provider)(nil)

// silence unused import in older AWS SDK layouts
var _ = aws.Credentials{}
