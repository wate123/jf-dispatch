package auth

import (
	"context"
	"crypto/subtle"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

type TokenCredentials struct{ Token string }

var _ credentials.PerRPCCredentials = TokenCredentials{}

func (t TokenCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + t.Token}, nil
}
func (TokenCredentials) RequireTransportSecurity() bool { return false }
func Unary(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		if e := check(ctx, token); e != nil {
			return nil, e
		}
		return h(ctx, req)
	}
}
func Stream(token string) grpc.StreamServerInterceptor {
	return func(s any, ss grpc.ServerStream, info *grpc.StreamServerInfo, h grpc.StreamHandler) error {
		if e := check(ss.Context(), token); e != nil {
			return e
		}
		return h(s, ss)
	}
}
func check(ctx context.Context, want string) error {
	if want == "" {
		return nil
	}
	md, _ := metadata.FromIncomingContext(ctx)
	got := strings.TrimPrefix(first(md.Get("authorization")), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid cluster token")
	}
	return nil
}
func first(v []string) string {
	if len(v) > 0 {
		return v[0]
	}
	return ""
}
