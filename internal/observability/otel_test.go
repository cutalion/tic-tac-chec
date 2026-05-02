package observability

import (
	"context"
	"testing"
	"tic-tac-chec/internal/web/config"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSetupLogs_Smoke(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	shutdown, err := SetupLogs(ctx, "test", &config.Logging{OtelEnabled: true, LogEnabled: false})
	require.NoError(t, err)

	require.NoError(t, shutdown(ctx))
}
