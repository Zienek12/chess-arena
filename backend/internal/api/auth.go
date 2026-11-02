package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// authorized reports whether r carries the API token, or no token is configured.
func (server *Server) authorized(r *http.Request) bool {
	if server.options.Token == "" {
		return true
	}
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return false
	}
	// Comparing hashes keeps the comparison constant-time regardless of length.
	given, expected := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(server.options.Token))
	return subtle.ConstantTimeCompare(given[:], expected[:]) == 1
}

// requireToken rejects requests without the API token. It guards every
// request that changes data or starts engines.
func (server *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !server.authorized(r) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="chess-arena"`)
			writeError(w, http.StatusUnauthorized, "a valid API token is required")
			return
		}
		next(w, r)
	}
}

type authStatus struct {
	TokenRequired bool `json:"token_required"`
	Authorized    bool `json:"authorized"`
}

// authStatus lets the frontend check whether writes need a token and whether
// the one it holds is valid.
func (server *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, authStatus{TokenRequired: server.options.Token != "", Authorized: server.authorized(r)})
}
