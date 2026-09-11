package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestOllamaSupportsResponsesCompact(t *testing.T) {
	assert.True(t, SupportsResponsesCompact(constant.ChannelTypeOllama, constant.APITypeOllama))
}
