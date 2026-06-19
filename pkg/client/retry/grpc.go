package retry

import (
	"context"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCRetryableCodes returns the gRPC status codes retried by the Hatchet client.
func GRPCRetryableCodes() []codes.Code {
	return []codes.Code{
		codes.ResourceExhausted,
		codes.DeadlineExceeded,
		codes.Internal,
		codes.Unavailable,
	}
}

// GRPCDialOptions returns gRPC dial options that install unary and stream retry interceptors.
// When enabled is false, it returns nil.
func GRPCDialOptions(l *zerolog.Logger, enabled bool) []grpc.DialOption {
	if !enabled {
		return nil
	}

	retryOnCodes := GRPCRetryableCodes()

	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponentialWithJitter(5*time.Second, 0.10)),
		grpc_retry.WithMax(5),
		grpc_retry.WithPerRetryTimeout(30 * time.Second),
		grpc_retry.WithCodes(retryOnCodes...),
		grpc_retry.WithOnRetryCallback(grpc_retry.OnRetryCallback(func(ctx context.Context, attempt uint, err error) {
			if containsCode(retryOnCodes, status.Code(err)) {
				l.Debug().Msgf("grpc_retry attempt: %d, backoff for %v", attempt, err)
			}
		})),
	}

	return []grpc.DialOption{
		grpc.WithChainStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpts...)),
		grpc.WithChainUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpts...)),
	}
}

func containsCode(codes []codes.Code, code codes.Code) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}
