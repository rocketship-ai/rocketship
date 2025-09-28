package auth

import (
	"encoding/json"
	"errors"
	"time"
)

// TokenData represents persisted OAuth tokens for a profile.
type TokenData struct {
	AccessToken    string    `json:"access_token"`
	RefreshToken   string    `json:"refresh_token,omitempty"`
	TokenType      string    `json:"token_type"`
	Expiry         time.Time `json:"expiry"`
	Scopes         []string  `json:"scopes,omitempty"`
	IDToken        string    `json:"id_token,omitempty"`
	Issuer         string    `json:"issuer,omitempty"`
	ClientID       string    `json:"client_id,omitempty"`
	Audience       string    `json:"audience,omitempty"`
	TokenEndpoint  string    `json:"token_endpoint,omitempty"`
	DeviceEndpoint string    `json:"device_endpoint,omitempty"`
}

// Marshal serialises the token payload.
func (t TokenData) Marshal() ([]byte, error) {
	return json.MarshalIndent(t, "", "  ")
}

// UnmarshalTokenData deserialises TokenData from JSON.
func UnmarshalTokenData(data []byte) (TokenData, error) {
	var td TokenData
	if err := json.Unmarshal(data, &td); err != nil {
		return TokenData{}, err
	}
	return td, nil
}

// IsExpired returns true if the access token has expired (with optional slack).
func (t TokenData) IsExpired(slack time.Duration) bool {
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().Add(slack).After(t.Expiry)
}

// HasRefresh returns true when a refresh token is stored.
func (t TokenData) HasRefresh() bool {
	return t.RefreshToken != ""
}

// Validate performs a minimal sanity check before persisting tokens.
func (t TokenData) Validate() error {
	if t.AccessToken == "" {
		return errors.New("access token is empty")
	}
	if t.TokenType == "" {
		return errors.New("token type is empty")
	}
	return nil
}
