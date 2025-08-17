package auth

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ctxKey int

const userIDKey ctxKey = 1

func UserIDFromCtx(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

var publicFullMethods = map[string]bool{
	"/pingerus.v1.AuthService/SignUp":  true,
	"/pingerus.v1.AuthService/SignIn":  true,
	"/pingerus.v1.AuthService/Refresh": true,
	"/pingerus.v1.AuthService/Logout":  true,
}

func UnaryAuthInterceptor(parse func(token string) (int64, error)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (interface{}, error) {
		if publicFullMethods[info.FullMethod] {
			return next(ctx, req)
		}

		token := bearer(ctx)
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		uid, err := parse(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		ctx = context.WithValue(ctx, userIDKey, uid)
		return next(ctx, req)
	}
}
