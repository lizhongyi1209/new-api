package sub2api

import (
	"io"

	"github.com/QuantumNous/new-api/dto"
)

func (a *Adaptor) GetRequestHeaders() (headers []string) {
	headers = []string{
		"Authorization",
		"Content-Type",
		"Accept",
		"OpenAI-Organization",
		"OpenAI-Beta",
		"User-Agent",
		"X-Request-Id",
	}
	return headers
}

func (a *Adaptor) ConvertSTTRequest(request *dto.AudioRequest) (io.Reader, error) {
	return nil, nil
}

func (a *Adaptor) ConvertTTSRequest(request *dto.AudioRequest) (any, error) {
	return request, nil
}
