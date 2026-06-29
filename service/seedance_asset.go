package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SeedanceAssetService handles creating and managing Seedance assets on ServiceInference
type SeedanceAssetService struct {
	BaseURL string
	APIKey  string
}

type AssetGroupResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreateAssetRequest struct {
	GroupID   string `json:"group_id"`
	URL       string `json:"url"`
	AssetType string `json:"asset_type"`
	Name      string `json:"name"`
}

type CreateAssetResponse struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Error  any    `json:"error,omitempty"`
}

type GetAssetResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  any    `json:"error,omitempty"`
}

// NewSeedanceAssetService creates a new service instance
func NewSeedanceAssetService(baseURL, apiKey string) *SeedanceAssetService {
	if baseURL == "" {
		baseURL = "https://model.service-inference.ai"
	}
	return &SeedanceAssetService{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
	}
}

// CreateAssetGroup creates a new asset group on ServiceInference
func (s *SeedanceAssetService) CreateAssetGroup(ctx context.Context, name, description string) (string, error) {
	body := map[string]string{
		"name":        name,
		"description": description,
	}

	var resp AssetGroupResponse
	if err := s.doRequest(ctx, "POST", "/v1/asset-groups", body, &resp); err != nil {
		return "", fmt.Errorf("failed to create asset group: %w", err)
	}

	if resp.ID == "" {
		return "", fmt.Errorf("asset group ID is empty")
	}

	return resp.ID, nil
}

// GetAssetGroup retrieves an asset group by ID
func (s *SeedanceAssetService) GetAssetGroup(ctx context.Context, groupID string) (*AssetGroupResponse, error) {
	var resp AssetGroupResponse
	if err := s.doRequest(ctx, "GET", "/v1/asset-groups/"+groupID, nil, &resp); err != nil {
		return nil, fmt.Errorf("failed to get asset group: %w", err)
	}
	return &resp, nil
}

// CreateAsset uploads an image and creates an asset
func (s *SeedanceAssetService) CreateAsset(ctx context.Context, groupID, imageURL, name string) (string, error) {
	// Step 1: Create asset
	req := CreateAssetRequest{
		GroupID:   groupID,
		URL:       imageURL,
		AssetType: "Image",
		Name:      name,
	}

	var createResp CreateAssetResponse
	if err := s.doRequest(ctx, "POST", "/v1/assets", req, &createResp); err != nil {
		return "", fmt.Errorf("failed to create asset: %w", err)
	}

	if createResp.ID == "" {
		return "", fmt.Errorf("asset ID is empty")
	}

	// Step 2: Wait for asset to be completed
	assetID, err := s.waitForAssetCompletion(ctx, createResp.ID, createResp.TaskID)
	if err != nil {
		return "", fmt.Errorf("asset creation failed: %w", err)
	}

	return assetID, nil
}

// waitForAssetCompletion polls the asset status until it's completed
func (s *SeedanceAssetService) waitForAssetCompletion(ctx context.Context, assetID, taskID string) (string, error) {
	maxAttempts := 30 // 30 attempts
	interval := 2 * time.Second

	for i := 0; i < maxAttempts; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		getReq := map[string]string{
			"asset_id": assetID,
			"task_id":  taskID,
		}

		var resp GetAssetResponse
		if err := s.doRequest(ctx, "POST", "/v1/assets/get", getReq, &resp); err != nil {
			// Log error but continue polling
			fmt.Printf("[WARN] Failed to get asset status (attempt %d/%d): %v\n", i+1, maxAttempts, err)
			time.Sleep(interval)
			continue
		}

		status := strings.ToLower(strings.TrimSpace(resp.Status))
		switch status {
		case "completed", "succeeded", "success":
			return resp.ID, nil
		case "failed", "failure", "error":
			errMsg := "unknown error"
			if resp.Error != nil {
				if errStr, ok := resp.Error.(string); ok {
					errMsg = errStr
				} else if errMap, ok := resp.Error.(map[string]interface{}); ok {
					if msg, exists := errMap["message"]; exists {
						errMsg = fmt.Sprintf("%v", msg)
					}
				}
			}
			return "", fmt.Errorf("asset creation failed: %s", errMsg)
		case "processing", "pending":
			// Continue waiting
		default:
			fmt.Printf("[WARN] Unknown asset status: %s\n", resp.Status)
		}

		time.Sleep(interval)
	}

	return "", fmt.Errorf("asset creation timeout after %d attempts", maxAttempts)
}

// doRequest makes an HTTP request to ServiceInference API
func (s *SeedanceAssetService) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := s.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
