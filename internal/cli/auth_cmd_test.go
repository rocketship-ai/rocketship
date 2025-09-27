package cli

import "testing"

func TestBuildFlowConfigFallbacks(t *testing.T) {
	profile := Profile{Name: "test", Auth: AuthProfile{ClientID: "client-123"}}
	info := &ServerInfo{
		AuthType:       "oidc",
		DeviceEndpoint: "https://device",
		TokenEndpoint:  "https://token",
		Scopes:         []string{"openid"},
	}
	cfg, err := buildFlowConfig(profile, info)
	if err != nil {
		t.Fatalf("buildFlowConfig failed: %v", err)
	}
	if cfg.ClientID != "client-123" {
		t.Fatalf("expected client id fallback, got %s", cfg.ClientID)
	}
	if len(cfg.Scopes) == 0 {
		t.Fatalf("expected scopes to be populated")
	}
}

func TestBuildFlowConfigMissingEndpoints(t *testing.T) {
	_, err := buildFlowConfig(Profile{}, &ServerInfo{AuthType: "oidc"})
	if err == nil {
		t.Fatalf("expected error due to missing endpoints")
	}
}

func TestDecodeIDToken(t *testing.T) {
	token := "eyJhbGciOiJub25lIn0." +
		"eyJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20ifQ." +
		""
	user, err := decodeIDToken(token)
	if err != nil {
		t.Fatalf("decodeIDToken failed: %v", err)
	}
	if user != "user@example.com" {
		t.Fatalf("expected email, got %s", user)
	}
}
