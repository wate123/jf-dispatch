package auth

import (
	"context"
	"google.golang.org/grpc/metadata"
	"testing"
)

func TestToken(t *testing.T) {
	good := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer secret"))
	if err := check(good, "secret"); err != nil {
		t.Fatal(err)
	}
	bad := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer wrong"))
	if err := check(bad, "secret"); err == nil {
		t.Fatal("expected rejection")
	}
}
