package servicekit

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Dial(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	dialOpts = append(dialOpts, opts...)
	return grpc.DialContext(ctx, address, dialOpts...)
}

func UnaryLoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		attrs := []any{
			"method", info.FullMethod,
			"duration", time.Since(start).String(),
		}
		if err != nil {
			attrs = append(attrs, "error", err)
			logger.Warn("grpc request failed", attrs...)
			return resp, err
		}
		logger.Info("grpc request", attrs...)
		return resp, nil
	}
}
