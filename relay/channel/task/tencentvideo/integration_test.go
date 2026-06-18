package tencentvideo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// TestIntegrationSubmitAndPoll performs a real end-to-end call against
// Tencent VCLM. It is skipped unless credentials are provided via env:
//
//	TC_SECRET_ID, TC_SECRET_KEY  — Tencent Cloud credentials (whitelisted)
//	TC_TEST_IMAGE_URL            — public image URL
//	TC_TEST_MODEL                — optional, default kling-v1-6
//	TC_TEST_REGION               — optional, default ap-guangzhou
//
// Run with:
//
//	go test ./relay/channel/task/tencentvideo/ -run TestIntegration -v
func TestIntegrationSubmitAndPoll(t *testing.T) {
	secretID := os.Getenv("TC_SECRET_ID")
	secretKey := os.Getenv("TC_SECRET_KEY")
	imageURL := os.Getenv("TC_TEST_IMAGE_URL")
	if secretID == "" || secretKey == "" || imageURL == "" {
		t.Skip("set TC_SECRET_ID, TC_SECRET_KEY and TC_TEST_IMAGE_URL to run the integration test")
	}
	region := tcDefaultRegion
	if r := os.Getenv("TC_TEST_REGION"); r != "" {
		region = r
	}
	modelCode := modelNameToTencentCode(getenvDefault("TC_TEST_MODEL", "kling-v1-6"))

	a := &TaskAdaptor{baseURL: "https://" + tcDefaultHost, region: region}
	client := &http.Client{Timeout: 30 * time.Second}

	// ---- submit ----
	submit := submitPayload{
		Model:    modelCode,
		Image:    &imageRef{Url: imageURL},
		Prompt:   "a gentle camera push-in, cinematic",
		Duration: getenvDefault("TC_TEST_DURATION", "5"),
		Mode:     getenvDefault("TC_TEST_MODE", "std"),
	}
	jobID := doCall(t, a, client, actionSubmit, submit, region, secretID, secretKey, func(body []byte) string {
		var sr submitResponse
		if err := common.Unmarshal(body, &sr); err != nil {
			t.Fatalf("unmarshal submit resp: %v\nbody=%s", err, body)
		}
		if sr.Response.Error != nil && sr.Response.Error.Code != "" {
			t.Fatalf("submit returned error: %s: %s", sr.Response.Error.Code, sr.Response.Error.Message)
		}
		if sr.Response.JobId == "" {
			t.Fatalf("empty JobId, body=%s", body)
		}
		return sr.Response.JobId
	})
	t.Logf("submitted JobId=%s", jobID)

	// ---- poll ----
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Second)
		body := rawCall(t, a, client, actionDescribe, map[string]string{"JobId": jobID}, region, secretID, secretKey)
		ti, err := a.ParseTaskResult(body)
		if err != nil {
			t.Fatalf("parse describe: %v\nbody=%s", err, body)
		}
		var dr describeResponse
		_ = common.Unmarshal(body, &dr)
		t.Logf("status=%s progress=%s FinalUnitDeduction=%q tokens=%d url=%s",
			ti.Status, ti.Progress, dr.Response.FinalUnitDeduction, ti.TotalTokens, ti.Url)
		if ti.Status == "SUCCESS" {
			if ti.Url == "" {
				t.Fatalf("success but no video url, body=%s", body)
			}
			t.Logf("DONE video=%s tokens=%d", ti.Url, ti.TotalTokens)
			return
		}
		if ti.Status == "FAILURE" {
			t.Fatalf("task failed: %s", ti.Reason)
		}
	}
	t.Fatal("polling timed out after 10m")
}

func getenvDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func rawCall(t *testing.T, a *TaskAdaptor, client *http.Client, action string, payload any, region, id, key string) []byte {
	t.Helper()
	data, err := common.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://%s/", a.host()), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	a.applyTC3Headers(req, action, data, id, key, region)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s HTTP %d: %s", action, resp.StatusCode, body)
	}
	return body
}

func doCall(t *testing.T, a *TaskAdaptor, client *http.Client, action string, payload any, region, id, key string, extract func([]byte) string) string {
	body := rawCall(t, a, client, action, payload, region, id, key)
	return extract(body)
}
