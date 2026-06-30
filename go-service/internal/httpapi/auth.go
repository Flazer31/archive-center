// Package httpapi provides HTTP handlers for the Archive Center Go shadow service.
// This file contains the authentication middleware skeleton.
package httpapi

import "net/http"

// authMiddleware returns an HTTP handler that optionally enforces bearer-token auth.
// When Auth.Enforce is false (the R0/R1 default), the middleware is a pass-through.
// When Enforce is true, a valid Authorization: Bearer <token> header is required.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.Cfg.Auth.Enforce {
			next.ServeHTTP(w, r)
			return
		}

		hdr := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(hdr) < len(prefix) || hdr[:len(prefix)] != prefix {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or malformed bearer token")
			return
		}
		token := hdr[len(prefix):]
		if token == "" || token != s.Cfg.Auth.BearerToken {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
