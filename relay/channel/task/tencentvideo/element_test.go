package tencentvideo

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateElementRequestMarshal locks the CreateAigcElement wire shape: the
// PascalCase keys Tencent expects, the nested ElementImageList structure, and
// the default Provider injection done by CreateElement.
func TestCreateElementRequestMarshal(t *testing.T) {
	req := &CreateElementRequest{
		Name:          "角色A",
		Description:   "一个穿红裙的女性",
		ReferenceType: ReferenceTypeImage,
		ElementImageList: &ElementImageList{
			FrontalImage: "https://example.com/front.jpg",
			ReferImages: []ReferImageItem{
				{ImageUrl: "https://example.com/1.jpg"},
				{ImageUrl: "https://example.com/2.jpg"},
			},
		},
		TagList: []TagItem{{TagId: "o_101"}},
	}

	data, err := common.Marshal(req)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, common.Unmarshal(data, &decoded))

	assert.Equal(t, "角色A", decoded["Name"])
	assert.Equal(t, "image_refer", decoded["ReferenceType"])

	il, ok := decoded["ElementImageList"].(map[string]any)
	require.True(t, ok, "ElementImageList must marshal as an object")
	assert.Equal(t, "https://example.com/front.jpg", il["FrontalImage"])
	refers, ok := il["ReferImages"].([]any)
	require.True(t, ok)
	assert.Len(t, refers, 2)

	// Provider absent here must be omitted (omitempty), not sent as null.
	_, present := decoded["Provider"]
	assert.False(t, present, "absent Provider must be omitted on marshal")
}
