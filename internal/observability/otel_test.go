package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSetupLogs_Smoke(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	shutdown, err := SetupLogs(ctx, "test")
	require.NoError(t, err)

	require.NoError(t, shutdown(ctx))
}
