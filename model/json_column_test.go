package model

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONColumnValuersReturnString(t *testing.T) {
	testCases := map[string]driver.Valuer{
		"channel_info": ChannelInfo{IsMultiKey: true, MultiKeySize: 2},
		"properties":   Properties{Input: "hello"},
		"private_data": TaskPrivateData{Key: "key"},
		"json_value":   JSONValue(`[{"key":"value"}]`),
	}

	for name, valuer := range testCases {
		t.Run(name, func(t *testing.T) {
			value, err := valuer.Value()
			require.NoError(t, err)
			_, ok := value.(string)
			assert.True(t, ok, "JSON column Valuer returned %T", value)
		})
	}
}

func TestJSONColumnScannersAcceptStringAndBytes(t *testing.T) {
	for _, useBytes := range []bool{true, false} {
		channelInfoValue := interface{}(`{"is_multi_key":true,"multi_key_size":2}`)
		propertiesValue := interface{}(`{"input":"hello"}`)
		privateDataValue := interface{}(`{"key":"key"}`)
		if useBytes {
			channelInfoValue = []byte(channelInfoValue.(string))
			propertiesValue = []byte(propertiesValue.(string))
			privateDataValue = []byte(privateDataValue.(string))
		}

		t.Run(map[bool]string{true: "bytes", false: "string"}[useBytes], func(t *testing.T) {
			var info ChannelInfo
			require.NoError(t, info.Scan(channelInfoValue))
			assert.True(t, info.IsMultiKey)
			assert.Equal(t, 2, info.MultiKeySize)

			var properties Properties
			require.NoError(t, properties.Scan(propertiesValue))
			assert.Equal(t, "hello", properties.Input)

			var privateData TaskPrivateData
			require.NoError(t, privateData.Scan(privateDataValue))
			assert.Equal(t, "key", privateData.Key)
		})
	}
}
