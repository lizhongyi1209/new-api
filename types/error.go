package types

import relaytypes "github.com/QuantumNous/new-api/relaykit/types"

// Keep the host API compatible while relay and host share one error identity.
type OpenAIError = relaytypes.OpenAIError
type ClaudeError = relaytypes.ClaudeError
type ErrorType = relaytypes.ErrorType
type ErrorCode = relaytypes.ErrorCode
type NewAPIError = relaytypes.NewAPIError
type NewAPIErrorOptions = relaytypes.NewAPIErrorOptions
type RawUpstreamResponse = relaytypes.RawUpstreamResponse

const (
	ErrorTypeNewAPIError = relaytypes.ErrorTypeNewAPIError
	ErrorTypeOpenAIError = relaytypes.ErrorTypeOpenAIError
	ErrorTypeClaudeError = relaytypes.ErrorTypeClaudeError
	ErrorTypeMidjourneyError = relaytypes.ErrorTypeMidjourneyError
	ErrorTypeGeminiError = relaytypes.ErrorTypeGeminiError
	ErrorTypeRerankError = relaytypes.ErrorTypeRerankError
	ErrorTypeUpstreamError = relaytypes.ErrorTypeUpstreamError

	ErrorCodeInvalidRequest = relaytypes.ErrorCodeInvalidRequest
	ErrorCodeSensitiveWordsDetected = relaytypes.ErrorCodeSensitiveWordsDetected
	ErrorCodeViolationFeeGrokCSAM = relaytypes.ErrorCodeViolationFeeGrokCSAM
	ErrorCodeCountTokenFailed = relaytypes.ErrorCodeCountTokenFailed
	ErrorCodeModelPriceError = relaytypes.ErrorCodeModelPriceError
	ErrorCodeInvalidApiType = relaytypes.ErrorCodeInvalidApiType
	ErrorCodeJsonMarshalFailed = relaytypes.ErrorCodeJsonMarshalFailed
	ErrorCodeDoRequestFailed = relaytypes.ErrorCodeDoRequestFailed
	ErrorCodeGetChannelFailed = relaytypes.ErrorCodeGetChannelFailed
	ErrorCodeGenRelayInfoFailed = relaytypes.ErrorCodeGenRelayInfoFailed
	ErrorCodeChannelNoAvailableKey = relaytypes.ErrorCodeChannelNoAvailableKey
	ErrorCodeChannelParamOverrideInvalid = relaytypes.ErrorCodeChannelParamOverrideInvalid
	ErrorCodeChannelHeaderOverrideInvalid = relaytypes.ErrorCodeChannelHeaderOverrideInvalid
	ErrorCodeChannelModelMappedError = relaytypes.ErrorCodeChannelModelMappedError
	ErrorCodeChannelAwsClientError = relaytypes.ErrorCodeChannelAwsClientError
	ErrorCodeChannelInvalidKey = relaytypes.ErrorCodeChannelInvalidKey
	ErrorCodeChannelResponseTimeExceeded = relaytypes.ErrorCodeChannelResponseTimeExceeded
	ErrorCodeReadRequestBodyFailed = relaytypes.ErrorCodeReadRequestBodyFailed
	ErrorCodeConvertRequestFailed = relaytypes.ErrorCodeConvertRequestFailed
	ErrorCodeAccessDenied = relaytypes.ErrorCodeAccessDenied
	ErrorCodeBadRequestBody = relaytypes.ErrorCodeBadRequestBody
	ErrorCodeReadResponseBodyFailed = relaytypes.ErrorCodeReadResponseBodyFailed
	ErrorCodeBadResponseStatusCode = relaytypes.ErrorCodeBadResponseStatusCode
	ErrorCodeBadResponse = relaytypes.ErrorCodeBadResponse
	ErrorCodeBadResponseBody = relaytypes.ErrorCodeBadResponseBody
	ErrorCodeEmptyResponse = relaytypes.ErrorCodeEmptyResponse
	ErrorCodeAwsInvokeError = relaytypes.ErrorCodeAwsInvokeError
	ErrorCodeModelNotFound = relaytypes.ErrorCodeModelNotFound
	ErrorCodePromptBlocked = relaytypes.ErrorCodePromptBlocked
	ErrorCodeQueryDataError = relaytypes.ErrorCodeQueryDataError
	ErrorCodeUpdateDataError = relaytypes.ErrorCodeUpdateDataError
	ErrorCodeInsufficientUserQuota = relaytypes.ErrorCodeInsufficientUserQuota
	ErrorCodePreConsumeTokenQuotaFailed = relaytypes.ErrorCodePreConsumeTokenQuotaFailed
)

var (
	NewError = relaytypes.NewError
	NewOpenAIError = relaytypes.NewOpenAIError
	InitOpenAIError = relaytypes.InitOpenAIError
	NewErrorWithStatusCode = relaytypes.NewErrorWithStatusCode
	WithOpenAIError = relaytypes.WithOpenAIError
	WithClaudeError = relaytypes.WithClaudeError
	IsChannelError = relaytypes.IsChannelError
	IsSkipRetryError = relaytypes.IsSkipRetryError
	ErrOptionWithSkipRetry = relaytypes.ErrOptionWithSkipRetry
	ErrOptionWithNoRecordErrorLog = relaytypes.ErrOptionWithNoRecordErrorLog
	ErrOptionWithStatusCode = relaytypes.ErrOptionWithStatusCode
	ErrOptionWithHideErrMsg = relaytypes.ErrOptionWithHideErrMsg
	IsRecordErrorLog = relaytypes.IsRecordErrorLog
)
