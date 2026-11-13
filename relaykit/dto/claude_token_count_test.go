package dto

import (
	"strings"
	"testing"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Claude Code returns screenshots and file reads inside tool_result blocks.
// Their base64 payload must be estimated as media, not tokenized as text,
// because the estimate becomes the bill when upstream reports no usage.
func TestClaudeTokenCountMetaCountsToolResultMediaAsFiles(t *testing.T) {
	imageData := strings.Repeat("iVBORw0KGgo", 1000)
	pdfData := strings.Repeat("JVBERi0xLjQK", 1000)
	raw := `{
		"model":"claude-test",
		"max_tokens":1024,
		"messages":[
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":"plain tool output"},
				{"type":"tool_result","tool_use_id":"toolu_2","content":[
					{"type":"text","text":"screenshot taken"},
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + imageData + `"}},
					{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"` + pdfData + `"}},
					{"type":"document","source":{"type":"text","media_type":"text/plain","data":"inline document text"}}
				]}
			]}
		]
	}`
	var req ClaudeRequest
	require.NoError(t, kitutil.Unmarshal([]byte(raw), &req))

	meta := req.GetTokenCountMeta()

	assert.NotContains(t, meta.CombineText, imageData)
	assert.NotContains(t, meta.CombineText, pdfData)
	assert.Contains(t, meta.CombineText, "plain tool output")
	assert.Contains(t, meta.CombineText, "screenshot taken")
	assert.Contains(t, meta.CombineText, "inline document text")
	require.Len(t, meta.Files, 2)
	assert.Equal(t, types.FileTypeImage, meta.Files[0].FileType)
	assert.Equal(t, types.FileTypeFile, meta.Files[1].FileType)
}
