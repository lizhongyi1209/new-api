package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupContextForSelectedChannelUsesRequestBillingGroup(t *testing.T) {
	tests := []struct {
		name          string
		selectedGroup string
	}{
		{name: "explicit token group", selectedGroup: "gpt-image-special"},
		{name: "resolved auto group", selectedGroup: "image-special"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			channel := &model.Channel{
				Id:    246,
				Name:  "multi-group-image-channel",
				Key:   "test-key",
				Group: "image-special,gpt-image-special",
			}

			apiErr := SetupContextForSelectedChannel(c, channel, "gpt-image-test", test.selectedGroup)

			require.Nil(t, apiErr)
			assert.Equal(t, test.selectedGroup, common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
			assert.NotEqual(t, channel.Group, common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		})
	}
}
