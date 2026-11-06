package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/tencentvod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func vodMJStringPtr(v string) *string { return &v }
func vodMJUintPtr(v uint) *uint       { return &v }
func vodMJIntPtr(v int) *int          { return &v }

// TestValidateGenerateImageVodMJRejectsUnsupportedParameters pins the contract:
// MJ takes its settings from prompt flags, so an OpenAI-style parameter cannot
// be honoured. Silently accepting one would misprice the request, because a
// "--hd" hidden in the prompt still raises the upstream tier.
func TestValidateGenerateImageVodMJRejectsUnsupportedParameters(t *testing.T) {
	base := func() *dto.GenerateImageRequest {
		return &dto.GenerateImageRequest{Model: "mj-v8.2", Prompt: "a cat --ar 1:1 --q 1"}
	}

	for _, tc := range []struct {
		name    string
		mutate  func(*dto.GenerateImageRequest)
		wantErr string
	}{
		{name: "plain prompt is accepted", mutate: func(*dto.GenerateImageRequest) {}},
		{
			name:    "n is rejected because the count is fixed",
			mutate:  func(r *dto.GenerateImageRequest) { r.N = vodMJUintPtr(4) },
			wantErr: "不支持 n 参数",
		},
		{
			name:    "size is rejected because resolution comes from the prompt",
			mutate:  func(r *dto.GenerateImageRequest) { r.Size = "1024x1024" },
			wantErr: "不支持 size 参数",
		},
		{
			name:   "size auto is tolerated",
			mutate: func(r *dto.GenerateImageRequest) { r.Size = "auto" },
		},
		{
			name:    "quality is rejected",
			mutate:  func(r *dto.GenerateImageRequest) { r.Quality = "high" },
			wantErr: "不支持 quality 参数",
		},
		{
			name:    "mask is rejected",
			mutate:  func(r *dto.GenerateImageRequest) { r.Mask = &dto.ImageReference{ImageURL: vodMJStringPtr("https://example.com/m.png")} },
			wantErr: "不支持 mask 参数",
		},
		{
			name:    "markup is rejected",
			mutate:  func(r *dto.GenerateImageRequest) { r.Markup = &dto.ImageReference{ImageURL: vodMJStringPtr("https://example.com/m.png")} },
			wantErr: "不支持 markup 参数",
		},
		{
			name:    "output format is rejected",
			mutate:  func(r *dto.GenerateImageRequest) { r.OutputFormat = vodMJStringPtr("png") },
			wantErr: "不支持 output_format 参数",
		},
		{
			name:    "frame timing is rejected",
			mutate:  func(r *dto.GenerateImageRequest) { r.FrameCount = vodMJIntPtr(9) },
			wantErr: "不支持 frame_count 参数",
		},
		{
			name: "two reference images are rejected",
			mutate: func(r *dto.GenerateImageRequest) {
				first, second := "https://example.com/a.png", "https://example.com/b.png"
				r.Images = []dto.GenerateImageInput{{Value: &first}, {Value: &second}}
			},
			wantErr: "只支持 1 张参考图",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := base()
			tc.mutate(req)
			err := validateGenerateImageVodMJ(req)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestVodMJModelVersionMapsTheGatewayName keeps the model name and the
// upstream version in sync: the billed tier depends on the version the
// upstream actually runs.
func TestVodMJModelVersionMapsTheGatewayName(t *testing.T) {
	assert.Equal(t, "v8.2", vodMJModelVersion("mj-v8.2"))
	assert.Equal(t, "v8.2", vodMJModelVersion("MJ-V8.2"))
	assert.Equal(t, "v8.1", vodMJModelVersion("mj-v8.1"))
	assert.Equal(t, "v8.2", vodMJModelVersion("unexpected"), "an unknown name falls back to the released version")
}

// TestVodMJSessionIDIsBoundedAndStable guards the deduplication key: VOD
// rejects a repeated key within three days and caps the field at 50 chars, so
// a long public task id must still produce a usable, deterministic value.
func TestVodMJSessionIDIsBoundedAndStable(t *testing.T) {
	task := &model.Task{TaskID: "task_0123456789abcdef0123456789abcdef"}
	id := vodMJSessionID(task)
	assert.LessOrEqual(t, len(id), 50)
	assert.Equal(t, id, vodMJSessionID(task), "the key must be stable across retries")

	assert.Equal(t, "mj-t1", vodMJSessionID(&model.Task{TaskID: "t1"}))

	long := vodMJSessionID(&model.Task{TaskID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	assert.LessOrEqual(t, len(long), 50)
}

// TestVodMJOutputFormatFallsBackToURL covers the observed upstream shape:
// FileType is empty for MJ, so the format has to come from the URL.
func TestVodMJOutputFormatFallsBackToURL(t *testing.T) {
	assert.Equal(t, "png", vodMJOutputFormat(tencentvod.OutputFileInfo{
		FileURL: "https://x/aigcImageGenFile_1.png",
	}))
	assert.Equal(t, "png", vodMJOutputFormat(tencentvod.OutputFileInfo{
		FileURL: "https://x/a.png?sign=abc",
	}))
	assert.Equal(t, "webp", vodMJOutputFormat(tencentvod.OutputFileInfo{
		FileURL: "https://x/a.webp", FileType: "WEBP",
	}), "a present FileType wins over the URL suffix")
}

// TestVodMJImagesKeepEveryResultAndRealSize guards two documented rules: all
// four images must be preserved, and the reported size comes from the upstream
// metadata rather than from the request.
func TestVodMJImagesKeepEveryResultAndRealSize(t *testing.T) {
	detail := tencentvod.TaskDetailResponse{}
	detail.AigcImageTask.Output.FileInfos = []tencentvod.OutputFileInfo{
		{FileURL: "https://x/1.png", MetaData: &tencentvod.OutputFileMetaData{Width: 1024, Height: 1024}},
		{FileURL: "https://x/2.png", MetaData: &tencentvod.OutputFileMetaData{Width: 1024, Height: 1024}},
		{FileURL: "https://x/3.png", MetaData: &tencentvod.OutputFileMetaData{Width: 1024, Height: 1024}},
		{FileURL: "https://x/4.png", MetaData: &tencentvod.OutputFileMetaData{Width: 1024, Height: 1024}},
		{FileURL: "  ", MetaData: &tencentvod.OutputFileMetaData{Width: 1024, Height: 1024}},
	}

	images := vodMJImages(detail)
	require.Len(t, images, 4, "an entry without a URL must be dropped, and the rest kept")
	assert.Equal(t, "https://x/1.png", images[0].Url)
	assert.Equal(t, "1024x1024", images[0].Size)
	assert.Equal(t, "png", images[0].OutputFormat)
}
