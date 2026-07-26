package setting

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const R2PublicUploadClientsOptionKey = "R2PublicUploadClientsSecret"

var publicUploadClientIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)

type R2PublicUploadClient struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Origins           []string `json:"origins"`
	Secret            string   `json:"secret"`
	Enabled           bool     `json:"enabled"`
	MaxFileSizeMB     int      `json:"max_file_size_mb"`
	RequestsPerMinute int      `json:"requests_per_minute"`
}

func NewDefaultR2PublicUploadClients() ([]R2PublicUploadClient, error) {
	mainSecret, err := common.GenerateRandomKey(48)
	if err != nil {
		return nil, err
	}
	thinksAPISecret, err := common.GenerateRandomKey(48)
	if err != nil {
		return nil, err
	}
	return []R2PublicUploadClient{
		{
			ID: "main", Name: "Main site", Enabled: true,
			Origins: []string{"https://api.o1key.cn", "https://cf-api.o1key.com", "https://api.o1key.com"},
			Secret:  mainSecret, MaxFileSizeMB: 100, RequestsPerMinute: 30,
		},
		{
			ID: "thinksapi", Name: "ThinksAPI", Enabled: true,
			Origins: []string{"https://api.thinksapi.com"},
			Secret:  thinksAPISecret, MaxFileSizeMB: 100, RequestsPerMinute: 20,
		},
	}, nil
}

func R2PublicUploadClientsToJSON(clients []R2PublicUploadClient) (string, error) {
	data, err := common.Marshal(clients)
	return string(data), err
}

func GetR2PublicUploadClients() []R2PublicUploadClient {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[R2PublicUploadClientsOptionKey]
	common.OptionMapRWMutex.RUnlock()
	var clients []R2PublicUploadClient
	if common.UnmarshalJsonStr(raw, &clients) != nil {
		return nil
	}
	return clients
}

func ValidateR2PublicUploadClients(clients []R2PublicUploadClient) error {
	if len(clients) > 50 {
		return fmt.Errorf("at most 50 downstream clients are allowed")
	}
	seenIDs := make(map[string]struct{}, len(clients))
	seenOrigins := make(map[string]string)
	for index := range clients {
		client := &clients[index]
		client.ID = strings.ToLower(strings.TrimSpace(client.ID))
		client.Name = strings.TrimSpace(client.Name)
		if !publicUploadClientIDPattern.MatchString(client.ID) {
			return fmt.Errorf("invalid downstream id %q", client.ID)
		}
		if _, exists := seenIDs[client.ID]; exists {
			return fmt.Errorf("duplicate downstream id %q", client.ID)
		}
		seenIDs[client.ID] = struct{}{}
		if client.Name == "" || len(client.Name) > 64 {
			return fmt.Errorf("downstream %q requires a name up to 64 characters", client.ID)
		}
		if len(client.Origins) == 0 || len(client.Origins) > 20 {
			return fmt.Errorf("downstream %q requires 1 to 20 origins", client.ID)
		}
		normalizedOrigins := make([]string, 0, len(client.Origins))
		for _, rawOrigin := range client.Origins {
			origin, err := NormalizeR2PublicUploadOrigin(rawOrigin)
			if err != nil {
				return fmt.Errorf("downstream %q: %w", client.ID, err)
			}
			if owner, exists := seenOrigins[origin]; exists && owner != client.ID {
				return fmt.Errorf("origin %q is already assigned to %q", origin, owner)
			}
			seenOrigins[origin] = client.ID
			normalizedOrigins = append(normalizedOrigins, origin)
		}
		sort.Strings(normalizedOrigins)
		client.Origins = normalizedOrigins
		if client.Secret == "" {
			return fmt.Errorf("downstream %q requires a signing secret", client.ID)
		}
		if client.MaxFileSizeMB < 1 || client.MaxFileSizeMB > 100 {
			return fmt.Errorf("downstream %q file limit must be between 1 and 100 MB", client.ID)
		}
		if client.RequestsPerMinute < 1 || client.RequestsPerMinute > 600 {
			return fmt.Errorf("downstream %q rate limit must be between 1 and 600", client.ID)
		}
	}
	return nil
}

func NormalizeR2PublicUploadOrigin(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin %q must be an HTTPS origin", raw)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("origin %q must not contain a path", raw)
	}
	return "https://" + strings.ToLower(parsed.Host), nil
}

func FindR2PublicUploadClient(clientID, origin string) (R2PublicUploadClient, bool) {
	normalizedOrigin, err := NormalizeR2PublicUploadOrigin(origin)
	if err != nil {
		return R2PublicUploadClient{}, false
	}
	for _, client := range GetR2PublicUploadClients() {
		if !client.Enabled || client.ID != clientID {
			continue
		}
		for _, allowedOrigin := range client.Origins {
			if allowedOrigin == normalizedOrigin {
				return client, true
			}
		}
	}
	return R2PublicUploadClient{}, false
}
