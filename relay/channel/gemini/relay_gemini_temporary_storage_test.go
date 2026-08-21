package gemini

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceInlineDataWithLocalTemporaryStorageURL(t *testing.T) {
	t.Setenv("TEMP_STORAGE_DIR", t.TempDir())
	t.Setenv("TEMP_STORAGE_PUBLIC_BASE_URL", "https://api.o1key.cn")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.o1key.cn/v1beta/models/test:generateContent", nil)
	response := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content: dto.GeminiChatContent{
				Parts: []dto.GeminiPart{{
					InlineData: &dto.GeminiInlineData{
						MimeType: "image/png",
						Data:     temporaryStorageTestPNGForGemini,
					},
				}},
			},
		}},
	}

	err := replaceInlineDataWithStorageURLs(c, &response, dto.ImageOutputStrategyLocalTemp)
	require.NoError(t, err)
	part := response.Candidates[0].Content.Parts[0]
	assert.Nil(t, part.InlineData)
	require.NotNil(t, part.FileData)
	assert.True(t, strings.HasPrefix(part.FileData.FileUri, "https://api.o1key.cn/tmp/output/"))
}

const temporaryStorageTestPNGForGemini = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
