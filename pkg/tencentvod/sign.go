// Package tencentvod calls Tencent Cloud VOD's AIGC image API
// (CreateAigcImageTask / DescribeTaskDetail) for the Midjourney model.
//
// The signature scheme is TC3-HMAC-SHA256, the same one the VCLM video
// adaptor uses; only the service, version and action names differ.
package tencentvod

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// ServiceVOD is the product identifier used in the credential scope.
	ServiceVOD = "vod"
	// VersionVOD is the API version. It is independent of the model version
	// the caller puts in ModelVersion.
	VersionVOD = "2018-07-17"

	// HostAPI serves text-to-image submission and every task query.
	HostAPI = "vod.tencentcloudapi.com"
	// HostGateway serves Base64 image-to-image submission. The ordinary
	// domain is not used for that request.
	HostGateway = "gateway.vod-qcloud.com"

	signAlgorithm   = "TC3-HMAC-SHA256"
	contentTypeJSON = "application/json; charset=utf-8"
)

// Credential is the Tencent Cloud secret pair. The channel key stores it as
// "SecretId|SecretKey", matching the existing Tencent video channel.
type Credential struct {
	SecretID  string
	SecretKey string
}

// ParseCredential splits a channel key in "SecretId|SecretKey" form.
func ParseCredential(key string) (Credential, error) {
	secretID, secretKey, found := strings.Cut(key, "|")
	if !found {
		return Credential{}, fmt.Errorf("invalid api_key, required format is SecretId|SecretKey")
	}
	secretID = strings.TrimSpace(secretID)
	secretKey = strings.TrimSpace(secretKey)
	if secretID == "" || secretKey == "" {
		return Credential{}, fmt.Errorf("invalid api_key, SecretId/SecretKey must not be empty")
	}
	return Credential{SecretID: secretID, SecretKey: secretKey}, nil
}

// signTarget is the per-call input to the signature.
type signTarget struct {
	Host    string
	Action  string
	Version string
	Region  string
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, message string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

// sign sets the TC3-HMAC-SHA256 headers on req. payload must be the exact
// bytes that will be written to the connection: the signature covers them.
func sign(req *http.Request, payload []byte, cred Credential, target signTarget, now time.Time) {
	now = now.UTC()
	timestamp := now.Unix()
	date := now.Format("2006-01-02")

	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		contentTypeJSON, target.Host, strings.ToLower(target.Action))
	signedHeaders := "content-type;host;x-tc-action"
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		canonicalHeaders,
		signedHeaders,
		sha256Hex(payload),
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, ServiceVOD)
	stringToSign := strings.Join([]string{
		signAlgorithm,
		strconv.FormatInt(timestamp, 10),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+cred.SecretKey), date)
	secretService := hmacSHA256(secretDate, ServiceVOD)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		signAlgorithm, cred.SecretID, credentialScope, signedHeaders, signature))
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Host", target.Host)
	req.Host = target.Host
	req.Header.Set("X-TC-Action", target.Action)
	req.Header.Set("X-TC-Version", target.Version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	if target.Region != "" {
		req.Header.Set("X-TC-Region", target.Region)
	}
}
