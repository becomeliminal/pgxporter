package db

import (
	"context"
	"testing"
)

func TestStaticAuthProvider(t *testing.T) {
	p := StaticAuthProvider{Pwd: "s3cret"}
	got, err := p.Password(context.Background(), "h", 5432, "u")
	if err != nil {
		t.Fatalf("Password: %v", err)
	}
	if got != "s3cret" {
		t.Errorf("got %q, want %q", got, "s3cret")
	}
}
