package authbroker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

// parseToken validates and parses a JWT token, checking issuer and audience
func (s *Server) parseToken(token string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}

	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		switch {
		case s.signer.rsaKey != nil:
			return &s.signer.rsaKey.PublicKey, nil
		case s.signer.ecKey != nil:
			return &s.signer.ecKey.PublicKey, nil
		default:
			return nil, errors.New("no signing key configured")
		}
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("token invalid")
	}

	if iss, _ := claims["iss"].(string); iss != s.cfg.Issuer {
		return nil, errors.New("issuer mismatch")
	}
	if aud := claims["aud"]; aud != nil {
		if !matchAudience(aud, s.cfg.Audience) {
			return nil, errors.New("audience mismatch")
		}
	}

	return claims, nil
}

// principalFromClaims extracts user principal information from JWT claims
func principalFromClaims(claims jwt.MapClaims) (brokerPrincipal, error) {
	userIDStr := stringClaim(claims["user_id"])
	if userIDStr == "" {
		if sub := stringClaim(claims["sub"]); strings.HasPrefix(sub, "user:") {
			userIDStr = strings.TrimPrefix(sub, "user:")
		}
	}
	if userIDStr == "" {
		return brokerPrincipal{}, errors.New("token missing user identifier")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return brokerPrincipal{}, errors.New("token contains invalid user identifier")
	}

	roles, err := stringSliceFromClaim(claims["roles"])
	if err != nil {
		return brokerPrincipal{}, fmt.Errorf("invalid roles claim: %w", err)
	}
	if len(roles) == 0 {
		return brokerPrincipal{}, errors.New("token missing roles")
	}

	principal := brokerPrincipal{
		UserID:   userID,
		Roles:    roles,
		Email:    stringClaim(claims["email"]),
		Name:     stringClaim(claims["name"]),
		Username: stringClaim(claims["preferred_username"]),
	}
	return principal, nil
}

// mintTokens creates a new access token and refresh token pair
func (s *Server) mintTokens(ctx context.Context, user persistence.User, roles []string, orgID uuid.UUID, scopes []string) (oauthTokenResponse, error) {
	now := time.Now().UTC()
	accessExpires := now.Add(s.cfg.AccessTokenTTL)
	refreshExpires := now.Add(s.cfg.RefreshTokenTTL)

	jti, err := generateRandomToken()
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to generate jti: %w", err)
	}

	claims := jwt.MapClaims{
		"iss":                s.cfg.Issuer,
		"aud":                s.cfg.Audience,
		"sub":                fmt.Sprintf("user:%s", user.ID.String()),
		"user_id":            user.ID.String(),
		"github_user_id":     user.GitHubUserID,
		"exp":                accessExpires.Unix(),
		"iat":                now.Unix(),
		"email":              user.Email,
		"email_verified":     true,
		"name":               user.Name,
		"preferred_username": user.Username,
		"scope":              joinScopes(scopes),
		"roles":              roles,
		"jti":                jti,
	}

	if orgID != uuid.Nil {
		claims["org_id"] = orgID.String()
	}

	accessToken, err := s.signer.Sign(claims)
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshToken, err := generateRandomToken()
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to mint refresh token: %w", err)
	}

	record := persistence.RefreshTokenRecord{
		TokenID:        uuid.New(),
		User:           user,
		OrganizationID: orgID,
		Scopes:         append([]string(nil), scopes...),
		IssuedAt:       now,
		ExpiresAt:      refreshExpires,
	}

	if err := s.store.SaveRefreshToken(ctx, refreshToken, record); err != nil {
		return oauthTokenResponse{}, fmt.Errorf("failed to persist refresh token: %w", err)
	}

	response := oauthTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		Scope:        joinScopes(scopes),
		IDToken:      accessToken,
	}
	return response, nil
}

// setAuthCookies sets secure httpOnly cookies for authentication
func (s *Server) setAuthCookies(w http.ResponseWriter, r *http.Request, tokens oauthTokenResponse) {
	// Determine if we should use Secure flag (HTTPS only)
	// Use Secure=true for production, but allow HTTP for local development
	secure := !strings.Contains(r.Host, "localhost") && !strings.Contains(r.Host, "127.0.0.1") && !strings.Contains(r.Host, ".local")

	// Set access token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    tokens.AccessToken,
		Path:     "/",
		MaxAge:   tokens.ExpiresIn,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	// Set refresh token cookie if present
	if tokens.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    tokens.RefreshToken,
			Path:     "/",
			MaxAge:   int(s.cfg.RefreshTokenTTL.Seconds()),
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}
