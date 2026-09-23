package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericOAuthAccessPolicyTemplatesEnforcedOnUserInfo(t *testing.T) {
	levelPolicy := `{"logic":"and","conditions":[{"field":"trust_level","op":"gte","value":2},{"field":"active","op":"eq","value":true}]}`
	organizationPolicy := `{"logic":"or","conditions":[{"field":"org","op":"eq","value":"core"},{"field":"roles","op":"contains","value":"admin"}]}`
	levelMessage := "Requires level {{required}}; your current level is {{current}} (field: {{field}})."
	organizationMessage := "Access is limited to approved organizations or roles. Organization: {{current.org}}; roles: {{current.roles}}."
	for _, scenario := range []struct {
		name            string
		policy          string
		profile         string
		allowed         bool
		message         string
		expectedMessage string
	}{
		{name: "level boundary and active", policy: levelPolicy, profile: `{"id":"42","trust_level":2,"active":true}`, allowed: true},
		{name: "insufficient level", policy: levelPolicy, profile: `{"id":"42","trust_level":1,"active":true}`, message: levelMessage, expectedMessage: "Requires level 2; your current level is 1 (field: trust_level)."},
		{name: "inactive", policy: levelPolicy, profile: `{"id":"42","trust_level":3,"active":false}`},
		{name: "missing level", policy: levelPolicy, profile: `{"id":"42","active":true}`},
		{name: "null level", policy: levelPolicy, profile: `{"id":"42","trust_level":null,"active":true}`},
		{name: "non numeric level", policy: levelPolicy, profile: `{"id":"42","trust_level":"unknown","active":true}`},
		{name: "NaN level", policy: levelPolicy, profile: `{"id":"42","trust_level":"NaN","active":true}`},
		{name: "infinite level", policy: levelPolicy, profile: `{"id":"42","trust_level":"+Inf","active":true}`},
		{name: "numeric string level", policy: levelPolicy, profile: `{"id":"42","trust_level":"2","active":true}`, allowed: true},
		{name: "explicit missing field rule", policy: `{"conditions":[{"field":"optional","op":"not_exists"}]}`, profile: `{"id":"42"}`, allowed: true},
		{name: "missing inequality field fails closed", policy: `{"conditions":[{"field":"required_attribute","op":"ne","value":"blocked"}]}`, profile: `{"id":"42"}`},
		{name: "missing activation", policy: levelPolicy, profile: `{"id":"42","trust_level":3}`},
		{name: "organization branch", policy: organizationPolicy, profile: `{"id":"42","org":"core","roles":["member"]}`, allowed: true},
		{name: "role branch", policy: organizationPolicy, profile: `{"id":"42","org":"other","roles":["member","admin"]}`, allowed: true},
		{name: "neither branch", policy: organizationPolicy, profile: `{"id":"42","org":"other","roles":["member"]}`, message: organizationMessage, expectedMessage: `Access is limited to approved organizations or roles. Organization: other; roles: ["member"].`},
		{name: "missing organization and roles", policy: organizationPolicy, profile: `{"id":"42"}`},
		{name: "invalid policy fails closed", policy: `{"logic":"unknown"}`, profile: `{"id":"42","trust_level":3,"active":true}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer synthetic-provider-token", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(scenario.profile))
				assert.NoError(t, err)
			}))
			t.Cleanup(server.Close)
			provider := NewGenericOAuthProvider(&model.CustomOAuthProvider{Name: "test-provider", Slug: "test-provider", UserInfoEndpoint: server.URL, UserIdField: "id", AccessPolicy: scenario.policy, AccessDeniedMessage: scenario.message})
			user, err := provider.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "synthetic-provider-token", TokenType: "Bearer"})
			if scenario.allowed {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.Equal(t, "42", user.ProviderUserID)
				return
			}
			require.Error(t, err)
			assert.Nil(t, user, "a denied provider profile must not reach login or account binding")
			if scenario.expectedMessage != "" {
				var denied *AccessDeniedError
				require.ErrorAs(t, err, &denied)
				assert.Equal(t, scenario.expectedMessage, denied.Message)
			}
		})
	}
}
