package authbroker

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

// handleOrgRegistrationStart initiates a new organization registration with email verification
func (s *Server) handleOrgRegistrationStart(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !principal.HasAnyRole("owner", "pending") {
		writeError(w, http.StatusForbidden, "owner or pending role required")
		return
	}
	if strings.TrimSpace(principal.Email) == "" {
		writeError(w, http.StatusBadRequest, "email address is required to start registration")
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "organization name is required")
		return
	}
	if len(name) > maxOrgNameLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("organization name must be <= %d characters", maxOrgNameLength))
		return
	}

	// Use provided email or fall back to principal's GitHub email
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = principal.Email
	}
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "valid email address is required")
		return
	}

	ctx := r.Context()
	if err := s.store.DeleteOrgRegistrationsForUser(ctx, principal.UserID); err != nil {
		log.Printf("failed to clear previous registrations: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to reset registration state")
		return
	}

	code, salt, hash, err := newVerificationSecret(verificationCodeLength)
	if err != nil {
		log.Printf("failed to generate verification code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to prepare verification code")
		return
	}

	now := s.nowUTC()
	reg := persistence.OrganizationRegistration{
		ID:                uuid.New(),
		UserID:            principal.UserID,
		Email:             email,
		OrgName:           name,
		CodeHash:          hash,
		CodeSalt:          salt,
		Attempts:          0,
		MaxAttempts:       maxRegistrationAttempts,
		ExpiresAt:         now.Add(orgRegistrationTTL),
		ResendAvailableAt: now.Add(orgRegistrationResendDelay),
	}

	rec, err := s.store.CreateOrgRegistration(ctx, reg)
	if err != nil {
		log.Printf("failed to persist registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to start registration")
		return
	}

	if err := s.mailer.SendOrgVerification(ctx, email, name, code, rec.ExpiresAt); err != nil {
		_ = s.store.DeleteOrgRegistration(ctx, rec.ID)
		log.Printf("failed to send verification email: %v", err)
		writeError(w, http.StatusBadGateway, "failed to send verification email")
		return
	}

	response := map[string]interface{}{
		"registration_id":     rec.ID.String(),
		"org_name":            rec.OrgName,
		"email":               rec.Email,
		"expires_at":          rec.ExpiresAt.Format(time.RFC3339),
		"resend_available_at": rec.ResendAvailableAt.Format(time.RFC3339),
	}
	writeJSON(w, http.StatusCreated, response)
}

// handleOrgRegistrationResend resends the verification code for an existing registration
func (s *Server) handleOrgRegistrationResend(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		RegistrationID string `json:"registration_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	regID, err := uuid.Parse(strings.TrimSpace(req.RegistrationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid registration id")
		return
	}

	ctx := r.Context()
	reg, err := s.store.GetOrgRegistration(ctx, regID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "registration not found")
			return
		}
		log.Printf("failed to load registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load registration")
		return
	}
	if reg.UserID != principal.UserID {
		writeError(w, http.StatusForbidden, "registration does not belong to caller")
		return
	}

	now := s.nowUTC()
	if reg.ExpiresAt.Before(now) {
		_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
		writeError(w, http.StatusGone, "registration expired")
		return
	}
	if reg.ResendAvailableAt.After(now) {
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("resend available after %s", reg.ResendAvailableAt.Format(time.RFC3339)))
		return
	}

	code, salt, hash, err := newVerificationSecret(verificationCodeLength)
	if err != nil {
		log.Printf("failed to generate resend code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate new code")
		return
	}

	updated, err := s.store.UpdateOrgRegistrationForResend(ctx, reg.ID, hash, salt, now.Add(orgRegistrationTTL), now.Add(orgRegistrationResendDelay))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "registration not found")
			return
		}
		log.Printf("failed to update registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update registration")
		return
	}

	if err := s.mailer.SendOrgVerification(ctx, updated.Email, updated.OrgName, code, updated.ExpiresAt); err != nil {
		log.Printf("failed to send verification email: %v", err)
		writeError(w, http.StatusBadGateway, "failed to send verification email")
		return
	}

	response := map[string]interface{}{
		"registration_id":     updated.ID.String(),
		"org_name":            updated.OrgName,
		"email":               updated.Email,
		"expires_at":          updated.ExpiresAt.Format(time.RFC3339),
		"resend_available_at": updated.ResendAvailableAt.Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, response)
}

// handleOrgRegistrationComplete verifies the code and completes organization registration
func (s *Server) handleOrgRegistrationComplete(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RegistrationID string `json:"registration_id"`
		Code           string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "verification code is required")
		return
	}

	regID, err := uuid.Parse(strings.TrimSpace(req.RegistrationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid registration id")
		return
	}

	ctx := r.Context()
	reg, err := s.store.GetOrgRegistration(ctx, regID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "registration not found")
			return
		}
		log.Printf("failed to load registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load registration")
		return
	}
	if reg.UserID != principal.UserID {
		writeError(w, http.StatusForbidden, "registration does not belong to caller")
		return
	}

	now := s.nowUTC()
	if reg.ExpiresAt.Before(now) {
		_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
		writeError(w, http.StatusGone, "registration expired")
		return
	}

	if !verifyCode(req.Code, reg.CodeSalt, reg.CodeHash) {
		_ = s.store.IncrementOrgRegistrationAttempts(ctx, reg.ID)
		if reg.Attempts+1 >= reg.MaxAttempts {
			_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
			writeError(w, http.StatusTooManyRequests, "verification failed too many times; restart registration")
			return
		}
		writeError(w, http.StatusUnauthorized, "verification code invalid")
		return
	}

	if err := s.store.UpdateUserEmail(ctx, principal.UserID, reg.Email); err != nil {
		if errors.Is(err, persistence.ErrEmailInUse) {
			writeError(w, http.StatusConflict, "email already associated with another account")
			return
		}
		log.Printf("failed to update user email: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update user email")
		return
	}

	var org persistence.Organization
	for attempt := 0; attempt < 5; attempt++ {
		slug, err := s.ensureUniqueSlug(ctx, reg.OrgName)
		if err != nil {
			log.Printf("failed to ensure slug: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to prepare organization")
			return
		}
		org, err = s.store.CreateOrganization(ctx, principal.UserID, reg.OrgName, slug)
		if err == nil {
			break
		}
		if errors.Is(err, persistence.ErrOrganizationSlugUsed) {
			continue
		}
		log.Printf("failed to create organization: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create organization")
		return
	}
	if org.ID == uuid.Nil {
		log.Printf("failed to allocate unique slug for %s", reg.OrgName)
		writeError(w, http.StatusConflict, "failed to reserve organization slug")
		return
	}

	if err := s.store.DeleteOrgRegistration(ctx, reg.ID); err != nil {
		log.Printf("failed to clear registration: %v", err)
	}

	response := map[string]interface{}{
		"organization": map[string]interface{}{
			"id":         org.ID.String(),
			"name":       org.Name,
			"slug":       org.Slug,
			"created_at": org.CreatedAt.Format(time.RFC3339),
		},
		"needs_claim_refresh": true,
	}
	writeJSON(w, http.StatusOK, response)
}
