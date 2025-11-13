package authbroker

import (
	"net/http"
)

// handleJWKS returns the JSON Web Key Set for token verification
func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	jwks, err := s.signer.JWKS()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to produce JWKS")
		return
	}
	writeJSON(w, http.StatusOK, jwks)
}

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
