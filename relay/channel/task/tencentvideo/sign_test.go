package tencentvideo

import (
	"net/http"
	"strings"
	"testing"
)

func TestSplitCredential(t *testing.T) {
	cases := []struct {
		key     string
		id, sk  string
		wantErr bool
	}{
		{"AKID123|secret456", "AKID123", "secret456", false},
		{" AKID123 | secret456 ", "AKID123", "secret456", false},
		{"onlyid", "", "", true},
		{"id|sk|extra", "", "", true},
		{"|sk", "", "", true},
		{"id|", "", "", true},
	}
	for _, tc := range cases {
		id, sk, err := splitCredential(tc.key)
		if tc.wantErr {
			if err == nil {
				t.Errorf("splitCredential(%q) expected error, got none", tc.key)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitCredential(%q) unexpected error: %v", tc.key, err)
			continue
		}
		if id != tc.id || sk != tc.sk {
			t.Errorf("splitCredential(%q) = (%q,%q), want (%q,%q)", tc.key, id, sk, tc.id, tc.sk)
		}
	}
}

func TestModelNameToTencentCode(t *testing.T) {
	cases := map[string]string{
		"kling-v1-t":          "v1.0",
		"kling-v1-6-t":        "v1.6",
		"kling-v2-1-master-t": "v2.1m",
		"kling-v3-t":          "v3.0",
		"KLING-V3-T":          "v3.0",
		"kling-v3":            "v3.0", // suffix optional; bare name still maps
		"v1.6":                "v1.6", // raw code passes through
		"unknown-model":       "unknown-model",
	}
	for in, want := range cases {
		if got := modelNameToTencentCode(in); got != want {
			t.Errorf("modelNameToTencentCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMotionControlModelRouting(t *testing.T) {
	// isMotionControlModel: only -motion(-t) names route to motion control
	motion := map[string]bool{
		"kling-v3-motion-t":   true,
		"kling-v2-6-motion-t": true,
		"kling-v3-motion":     true,
		"KLING-V3-MOTION-T":   true,
		"kling-v3-t":          false,
		"kling-v2-6-t":        false,
		"kling-v3":            false,
	}
	for in, want := range motion {
		if got := isMotionControlModel(in); got != want {
			t.Errorf("isMotionControlModel(%q) = %v, want %v", in, got, want)
		}
	}

	// modelNameToMotionModel: full names (kling-v2-6 / kling-v3), suffixes stripped
	mapping := map[string]string{
		"kling-v3-motion-t":   "kling-v3",
		"kling-v2-6-motion-t": "kling-v2-6",
		"kling-v3-motion":     "kling-v3",
		"KLING-V3-MOTION-T":   "kling-v3",
	}
	for in, want := range mapping {
		if got := modelNameToMotionModel(in); got != want {
			t.Errorf("modelNameToMotionModel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyTC3Headers(t *testing.T) {
	a := &TaskAdaptor{baseURL: "https://vclm.tencentcloudapi.com"}
	req, _ := http.NewRequest(http.MethodPost, "https://vclm.tencentcloudapi.com/", nil)
	payload := []byte(`{"JobId":"123"}`)

	a.applyTC3Headers(req, actionDescribe, payload, "AKIDexample", "secretexample", "ap-guangzhou")

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "TC3-HMAC-SHA256 ") {
		t.Errorf("Authorization not TC3: %q", auth)
	}
	for _, want := range []string{"Credential=AKIDexample/", "/vclm/tc3_request", "SignedHeaders=content-type;host;x-tc-action", "Signature="} {
		if !strings.Contains(auth, want) {
			t.Errorf("Authorization missing %q: %q", want, auth)
		}
	}
	if got := req.Header.Get("X-TC-Action"); got != actionDescribe {
		t.Errorf("X-TC-Action = %q, want %q", got, actionDescribe)
	}
	if got := req.Header.Get("X-TC-Version"); got != tcVersion {
		t.Errorf("X-TC-Version = %q, want %q", got, tcVersion)
	}
	if got := req.Header.Get("X-TC-Region"); got != "ap-guangzhou" {
		t.Errorf("X-TC-Region = %q, want ap-guangzhou", got)
	}
	if req.Host != "vclm.tencentcloudapi.com" {
		t.Errorf("req.Host = %q", req.Host)
	}
	if req.Header.Get("X-TC-Timestamp") == "" {
		t.Error("X-TC-Timestamp not set")
	}
}
