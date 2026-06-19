package retry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

func TestGRPCRetryableCodes(t *testing.T) {
	t.Parallel()

	retryable := GRPCRetryableCodes()
	assert.Contains(t, retryable, codes.ResourceExhausted)
	assert.Contains(t, retryable, codes.DeadlineExceeded)
	assert.Contains(t, retryable, codes.Internal)
	assert.Contains(t, retryable, codes.Unavailable)
	assert.NotContains(t, retryable, codes.FailedPrecondition)
}
