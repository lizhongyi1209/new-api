package console_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConsoleSettingsCountsBrowserCharacters(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		wantError   bool
	}{
		{name: "multibyte characters at limit", description: strings.Repeat("界", 200)},
		{name: "multibyte characters above limit", description: strings.Repeat("界", 201), wantError: true},
		{name: "surrogate pairs at limit", description: strings.Repeat("😀", 100)},
		{name: "surrogate pairs above limit", description: strings.Repeat("😀", 101), wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := common.Marshal([]map[string]interface{}{{
				"url":         "https://example.com/v1",
				"route":       "primary",
				"description": testCase.description,
				"color":       "blue",
			}})
			require.NoError(t, err)

			err = ValidateConsoleSettings(string(payload), "ApiInfo")
			if testCase.wantError {
				assert.ErrorContains(t, err, "说明长度不能超过200字符")
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidateConsoleSettingsAnnouncementExtraLimit(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		extra     string
		wantError bool
	}{
		{name: "at limit", extra: strings.Repeat("a", 100)},
		{name: "above limit", extra: strings.Repeat("a", 101), wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := common.Marshal([]map[string]interface{}{{
				"content":     "maintenance",
				"publishDate": "2026-08-17T00:00:00Z",
				"extra":       testCase.extra,
			}})
			require.NoError(t, err)

			err = ValidateConsoleSettings(string(payload), "Announcements")
			if testCase.wantError {
				assert.ErrorContains(t, err, "说明长度不能超过100字符")
				return
			}
			assert.NoError(t, err)
		})
	}
}
