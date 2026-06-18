package tencentvideo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// splitCredential parses the channel key in "SecretId|SecretKey" form.
func splitCredential(key string) (secretId, secretKey string, err error) {
	parts := strings.Split(key, "|")
	if len(parts) != 2 {
		return "", "", errors.New("invalid api_key, required format is SecretId|SecretKey")
	}
	secretId = strings.TrimSpace(parts[0])
	secretKey = strings.TrimSpace(parts[1])
	if secretId == "" || secretKey == "" {
		return "", "", errors.New("invalid api_key, SecretId/SecretKey must not be empty")
	}
	return secretId, secretKey, nil
}

func sha256hex(s string) string {
	b := sha256.Sum256([]byte(s))
	return hex.EncodeToString(b[:])
}

func hmacSha256(s, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(s))
	return string(h.Sum(nil))
}

// applyTC3Headers signs the request payload with TC3-HMAC-SHA256 and sets the
// required Tencent Cloud common headers on req.
// Reference: https://cloud.tencent.com/document/api/1616/107789
func (a *TaskAdaptor) applyTC3Headers(req *http.Request, action string, payload []byte, secretId, secretKey, region string) {
	host := a.host()
	now := time.Now().UTC()
	timestamp := now.Unix()
	date := now.Format("2006-01-02")

	// 1. canonical request
	httpRequestMethod := http.MethodPost
	canonicalURI := "/"
	canonicalQueryString := ""
	lowerAction := strings.ToLower(action)
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		"application/json; charset=utf-8", host, lowerAction)
	signedHeaders := "content-type;host;x-tc-action"
	hashedRequestPayload := sha256hex(string(payload))
	canonicalRequest := strings.Join([]string{
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload,
	}, "\n")

	// 2. string to sign
	algorithm := "TC3-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, tcService)
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := strings.Join([]string{
		algorithm,
		strconv.FormatInt(timestamp, 10),
		credentialScope,
		hashedCanonicalRequest,
	}, "\n")

	// 3. signature
	secretDate := hmacSha256(date, "TC3"+secretKey)
	secretService := hmacSha256(tcService, secretDate)
	secretSigning := hmacSha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(hmacSha256(string2sign, secretSigning)))

	// 4. authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, secretId, credentialScope, signedHeaders, signature)

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Host = host
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", tcVersion)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	if region != "" {
		req.Header.Set("X-TC-Region", region)
	}
}
