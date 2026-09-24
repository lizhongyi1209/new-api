package helper

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

var taskBillingParamPath = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(?:\.(?:[A-Za-z][A-Za-z0-9_]*|[0-9]+))*$`)

// FreezeTaskBillingExprRequestInput retains only the scalar request fields
// referenced by a plugin task expression. Complete task bodies can contain
// prompts, credentials and media, so they must not enter a billing snapshot.
func FreezeTaskBillingExprRequestInput(c *gin.Context, expression string) (billingexpr.RequestInput, error) {
	value, exists := c.Get("task_request")
	request, ok := value.(map[string]any)
	if !exists || !ok {
		return billingexpr.RequestInput{}, fmt.Errorf("task plugin request is unavailable for billing")
	}
	paramNames, err := billingexpr.ReferencedParams(expression)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	headerNames, err := billingexpr.ReferencedHeaders(expression)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	if len(paramNames) > 32 || len(headerNames) > 32 {
		return billingexpr.RequestInput{}, fmt.Errorf("too many task billing request references")
	}
	input := billingexpr.RequestInput{
		Headers: make(map[string]string, len(headerNames)),
		Params:  make(map[string]any, len(paramNames)),
		At:      time.Now().UTC().Truncate(time.Second),
	}
	for _, path := range paramNames {
		if len(path) > 128 || !taskBillingParamPath.MatchString(path) {
			return billingexpr.RequestInput{}, fmt.Errorf("task billing parameter path is invalid")
		}
		segments := strings.Split(path, ".")
		for _, segment := range segments {
			lower := strings.ToLower(segment)
			for _, sensitive := range []string{"authorization", "cookie", "api_key", "apikey", "token", "secret", "password", "credential", "base64"} {
				if strings.Contains(lower, sensitive) {
					return billingexpr.RequestInput{}, fmt.Errorf("task billing parameter path references private content")
				}
			}
			switch lower {
			case "prompt", "input", "output", "image", "images", "audio", "video", "url", "content", "messages", "files", "file", "data":
				return billingexpr.RequestInput{}, fmt.Errorf("task billing parameter path references private content")
			}
		}
		var current any = request
		for _, segment := range segments {
			switch item := current.(type) {
			case map[string]any:
				current = item[segment]
			case []any:
				index, indexErr := strconv.Atoi(segment)
				if indexErr != nil || index < 0 || index >= len(item) {
					current = nil
					break
				}
				current = item[index]
			default:
				current = nil
			}
			if current == nil {
				break
			}
		}
		switch scalar := current.(type) {
		case nil:
		case bool:
			input.Params[path] = scalar
		case string:
			if len(scalar) > 256 || strings.ContainsAny(scalar, "\r\n") {
				return billingexpr.RequestInput{}, fmt.Errorf("task billing parameter is too large")
			}
			input.Params[path] = scalar
		case float64:
			if math.IsNaN(scalar) || math.IsInf(scalar, 0) {
				return billingexpr.RequestInput{}, fmt.Errorf("task billing parameter is not finite")
			}
			input.Params[path] = scalar
		default:
			return billingexpr.RequestInput{}, fmt.Errorf("task billing parameter must be scalar")
		}
	}
	for _, name := range headerNames {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || len(name) > 128 || strings.ContainsAny(name, "\r\n") {
			return billingexpr.RequestInput{}, fmt.Errorf("task billing header name is invalid")
		}
		for _, sensitive := range []string{"authorization", "cookie", "api-key", "apikey", "api_key", "token", "secret", "password", "session", "credential", "x-auth"} {
			if strings.Contains(name, sensitive) {
				return billingexpr.RequestInput{}, fmt.Errorf("credential headers cannot be persisted for task billing")
			}
		}
		value := c.GetHeader(name)
		if len(value) > 256 || strings.ContainsAny(value, "\r\n") {
			return billingexpr.RequestInput{}, fmt.Errorf("task billing header value is too large")
		}
		input.Headers[name] = value
	}
	return input, nil
}

func ResolveIncomingBillingExprRequestInput(c *gin.Context, info *relaycommon.RelayInfo) (billingexpr.RequestInput, error) {
	if info != nil && info.BillingRequestInput != nil {
		input := cloneRequestInput(*info.BillingRequestInput)
		merged := cloneStringMap(info.RequestHeaders)
		maps.Copy(merged, input.Headers)
		input.Headers = merged
		return input, nil
	}

	input := billingexpr.RequestInput{}
	if info != nil {
		input.Headers = cloneStringMap(info.RequestHeaders)
	}

	bodyBytes, err := readIncomingBillingExprBody(c)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	input.Body = bodyBytes
	return input, nil
}

// ResolveImageBillingRequestInput freezes only validated scalar image
// parameters needed by pricing. Prompts, images, and base64 payloads are not
// copied into the derived billing body.
func ResolveImageBillingRequestInput(c *gin.Context, info *relaycommon.RelayInfo, input billingexpr.RequestInput) (billingexpr.RequestInput, error) {
	request, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return input, nil
	}
	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	if info.ChannelMeta != nil {
		channelType = info.ChannelType
	}
	count, err := request.ImageCount(channelType == constant.ChannelTypeAli)
	if err != nil {
		return input, err
	}
	topLevelCount, err := request.ImageCount(false)
	if err != nil {
		return input, err
	}
	body := map[string]any{
		"model":   request.Model,
		"n":       topLevelCount,
		"size":    request.Size,
		"quality": request.Quality,
	}
	if request.BillingParameters != nil {
		body["parameters"] = request.BillingParameters
	}
	encoded, err := common.Marshal(body)
	if err != nil {
		return input, err
	}
	input.Body = encoded
	input.ImageCount = &count
	return input, nil
}

func BuildBillingExprRequestInputFromRequest(request dto.Request, headers map[string]string) (billingexpr.RequestInput, error) {
	input := billingexpr.RequestInput{
		Headers: cloneStringMap(headers),
	}
	if request == nil {
		return input, nil
	}

	bodyBytes, err := common.Marshal(request)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	input.Body = bodyBytes
	return input, nil
}

func readIncomingBillingExprBody(c *gin.Context) ([]byte, error) {
	if c == nil || c.Request == nil || !isJSONContentType(c.Request.Header.Get("Content-Type")) {
		return nil, nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	return storage.Bytes()
}

func cloneRequestInput(src billingexpr.RequestInput) billingexpr.RequestInput {
	input := billingexpr.RequestInput{
		Headers: cloneStringMap(src.Headers),
		At:      src.At,
	}
	if src.Params != nil {
		input.Params = make(map[string]any, len(src.Params))
		for key, value := range src.Params {
			input.Params[key] = value
		}
	}
	if src.ImageCount != nil {
		count := *src.ImageCount
		input.ImageCount = &count
	}
	if src.ImageCount != nil {
		count := *src.ImageCount
		input.ImageCount = &count
	}
	if len(src.Body) > 0 {
		input.Body = append([]byte(nil), src.Body...)
	}
	return input
}

func isJSONContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(contentType, "application/json")
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		if strings.TrimSpace(key) == "" {
			continue
		}
		dst[key] = value
	}
	return dst
}
