package ali

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestRequestOpenAI2AliTopP(t *testing.T) {
	tests := []struct {
		name string
		topP *float64
		want *float64
	}{
		{name: "omitted", topP: nil, want: nil},
		{name: "in range", topP: lo.ToPtr(0.8), want: lo.ToPtr(0.8)},
		{name: "one", topP: lo.ToPtr(1.0), want: lo.ToPtr(0.99)},
		{name: "above one", topP: lo.ToPtr(1.5), want: lo.ToPtr(0.99)},
		{name: "zero", topP: lo.ToPtr(0.0), want: lo.ToPtr(0.01)},
		{name: "negative", topP: lo.ToPtr(-0.3), want: lo.ToPtr(0.01)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := requestOpenAI2Ali(dto.GeneralOpenAIRequest{
				Model: "qwen-plus",
				TopP:  test.topP,
			}, "qwen-plus")
			assert.Equal(t, test.want, got.TopP)
		})
	}
}
