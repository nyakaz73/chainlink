package functions_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink/v2/core/internal/testutils"
	"github.com/smartcontractkit/chainlink/v2/core/logger"
	"github.com/smartcontractkit/chainlink/v2/core/services/gateway/config"
	"github.com/smartcontractkit/chainlink/v2/core/services/gateway/handlers/functions"
)

func TestFunctionsHandler_Basic(t *testing.T) {
	t.Parallel()

	handler, err := functions.NewFunctionsHandler(json.RawMessage("{}"), &config.DONConfig{}, nil, nil, logger.TestLogger(t))
	require.NoError(t, err)

	// nil message
	err = handler.HandleUserMessage(testutils.Context(t), nil, nil)
	require.Error(t, err)
}
