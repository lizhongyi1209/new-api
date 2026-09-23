package jsplugin

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// The host uses ParseResponse for plugin submissions so it can persist the
// task before replying to the client. Keep the legacy method explicit: using
// it would discard the plugin state and the immediate result.
func (a *TaskAdaptor) DoResponse(_ *gin.Context, _ *http.Response, _ *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	return "", nil, service.TaskErrorWrapperLocal(fmt.Errorf("plugin submission requires the parsed response path"), "plugin_submit_unavailable", http.StatusInternalServerError)
}

func (a *TaskAdaptor) FetchTask(_ string, _ string, _ map[string]any, _ string) (*http.Response, error) {
	return nil, fmt.Errorf("plugin polling requires the persisted task context")
}

func (a *TaskAdaptor) ParseTaskResult(_ []byte) (*relaycommon.TaskInfo, error) {
	return nil, fmt.Errorf("plugin polling requires the persisted task context")
}

var _ channel.TaskAdaptor = (*TaskAdaptor)(nil)
var _ channel.TaskSubmitResponseParser = (*TaskAdaptor)(nil)
var _ channel.TaskContextPoller = (*TaskAdaptor)(nil)
var _ channel.OpenAIVideoConverter = (*TaskAdaptor)(nil)
var _ channel.TaskArtifactProvider = (*TaskAdaptor)(nil)
var _ channel.TaskContentRequestProvider = (*TaskAdaptor)(nil)
var _ channel.TaskUsageFactsProvider = (*TaskAdaptor)(nil)
var _ channel.TaskValidatedBillingProvider = (*TaskAdaptor)(nil)
var _ channel.TaskValidatedUsageFactsProvider = (*TaskAdaptor)(nil)
