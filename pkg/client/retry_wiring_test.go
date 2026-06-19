package client

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hatchet-dev/hatchet/pkg/client/retry"
)

func TestSDKRetryEnabledWiring(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		noRetry     bool
		noGrpcRetry bool
		grpcRetry   bool
		restRetry   bool
	}{
		{name: "defaults", grpcRetry: true, restRetry: true},
		{name: "no grpc only", noGrpcRetry: true, grpcRetry: false, restRetry: true},
		{name: "no retry all", noRetry: true, grpcRetry: false, restRetry: false},
		{name: "both set", noRetry: true, noGrpcRetry: true, grpcRetry: false, restRetry: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			grpcRetry, restRetry := sdkRetryEnabled(tc.noRetry, tc.noGrpcRetry)
			assert.Equal(t, tc.grpcRetry, grpcRetry)
			assert.Equal(t, tc.restRetry, restRetry)
		})
	}
}

func TestNewRestHTTPDoerWiring(t *testing.T) {
	t.Parallel()

	doer := newRestHTTPDoer()
	assert.IsType(t, &retry.RestDoer{}, doer)
}
