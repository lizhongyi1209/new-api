package serviceinference

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestMissingSeedanceAssetRequiresExplicitFailureAndKnownSource(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{PrivateData: model.TaskPrivateData{SeedanceAssetRecovery: &model.SeedanceAssetRecovery{
		Sources: map[string]model.SeedanceAssetRecreateRequest{"asset-known": {}, "mva-known": {}},
	}}}
	for _, test := range []struct {
		reason string
		want   bool
	}{
		{reason: "The specified asset asset-known is not found", want: true},
		{reason: "The specified asset mva-known is not found", want: true},
		{reason: "The specified asset asset-unknown is not found"},
		{reason: "The asset asset-known was rejected by moderation"},
		{reason: "Task not found"},
		{reason: "Invalid duration"},
		{reason: "asset not found"},
	} {
		t.Run(test.reason, func(t *testing.T) {
			assert.Equal(t, test.want, adaptor.MissingSeedanceAsset(task, test.reason))
		})
	}
	assert.False(t, adaptor.MissingSeedanceAsset(&model.Task{}, "The specified asset asset-known is not found"))
}
