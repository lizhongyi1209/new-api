package controller

import (
	"bytes"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestBuildUsageLogCSVPreservesReconciliationFields(t *testing.T) {
	data, err := buildUsageLogCSV([]*model.Log{{
		CreatedAt:        1700000000,
		Type:             model.LogTypeConsume,
		Username:         "user,one",
		Group:            "default",
		ModelName:        "seedance-2.0-mini",
		PromptTokens:     12,
		CompletionTokens: 34,
		Quota:            500000,
		RequestId:        "req-1",
	}})
	require.NoError(t, err)

	text := string(data)
	assert.True(t, strings.HasPrefix(text, "\xEF\xBB\xBFTime,Type"))
	assert.Contains(t, text, `"user,one"`)
	assert.Contains(t, text, ",12,34,46,500000,")
	assert.Contains(t, text, "seedance-2.0-mini")
}

func TestBuildUsageLogCSVNeutralizesSpreadsheetFormula(t *testing.T) {
	data, err := buildUsageLogCSV([]*model.Log{{Username: "=HYPERLINK(\"bad\")"}})
	require.NoError(t, err)
	assert.Contains(t, string(data), "'=HYPERLINK")
}

func TestBuildUsageLogStatementContainsNumericTokensAndTotal(t *testing.T) {
	data, err := buildUsageLogStatement([]*model.Log{{PromptTokens: 12, CompletionTokens: 34}})
	require.NoError(t, err)

	file, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer file.Close()
	assert.Equal(t, "12", mustCellValue(t, file, "Statement", "F5"))
	assert.Equal(t, "46", mustCellValue(t, file, "Statement", "H5"))
	formula, err := file.GetCellFormula("Statement", "H6")
	require.NoError(t, err)
	assert.Equal(t, "SUM(H5:H5)", formula)
}

func mustCellValue(t *testing.T, file *excelize.File, sheet string, cell string) string {
	t.Helper()
	value, err := file.GetCellValue(sheet, cell)
	require.NoError(t, err)
	return value
}
