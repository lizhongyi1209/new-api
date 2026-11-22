package relay

import (
	"errors"
	"net/http"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
)

func newAPIErrorFromParamOverride(err error) *types.NewAPIError {
	if fixedErr, ok := relaycommon.AsParamOverrideReturnError(err); ok {
		return relaycommon.NewAPIErrorFromParamOverride(fixedErr)
	}
	var valueErr *relaycommon.ParamOverrideRequestValueError
	if errors.As(err, &valueErr) {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	return types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
}
