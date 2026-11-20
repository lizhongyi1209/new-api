package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKlingPluginViewKeepsRetailResultAndPrivateSettlementApart(t *testing.T) {
	for _, raw := range []string{
		`{"data":{"task_id":"internal-id","final_unit_deduction":3.6,"task_result":{"videos":[{"url":"https://example.com/out","duration":"4.041"}]}}}`,
		`{"data":[{"id":"internal-id","outputs":[{"type":"video","url":"https://example.com/out"}],"billing":[{"charge_type":"cash","amount":3.6}]}]}`,
	} {
		task := &model.Task{TaskID: "public-id", Data: []byte(raw)}
		task.Properties.OriginModelName = "kling-3.0-omni"
		task.PrivateData.UpstreamTaskID = "internal-id"
		view, err := BuildTaskPluginView(task)
		require.NoError(t, err)
		out, err := common.Marshal(view.Data)
		require.NoError(t, err)
		assert.Contains(t, string(out), "public-id")
		assert.Contains(t, string(out), "https://example.com/out")
		assert.NotContains(t, string(out), "billing")
		assert.NotContains(t, string(out), "final_unit_deduction")
		assert.Equal(t, raw, string(task.Data))
	}
}
