package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store holds the DB pool for catalog handlers.
type Store struct {
	Pool *pgxpool.Pool
}

// New returns a Store backed by pool (may be nil in unit tests for pure funcs).
func New(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

// Auth hooks let the base router inject its JWT-backed identity lookup
// (internal/auth uses private context keys, so plain ctx.Value misses it).
// Set by internal/http/mounts.go; fall back to header lookup when nil.
var UserIDFromRequest func(*http.Request) string
var UserRoleFromRequest func(*http.Request) string

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

// userFromRequest extracts the caller's user id, compatible with the base auth
// middleware (context keys) with X-User / X-User-ID header fallback.
func userFromRequest(r *http.Request) string {
	if UserIDFromRequest != nil {
		if s := UserIDFromRequest(r); s != "" {
			return s
		}
	}
	ctx := r.Context()
	for _, k := range []any{"user_id", "userID", "uid", "sub"} {
		if v := ctx.Value(k); v != nil {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	// claims-style maps set by JWT middleware
	for _, k := range []any{"user", "claims"} {
		if v := ctx.Value(k); v != nil {
			if m, ok := v.(map[string]any); ok {
				for _, f := range []string{"user_id", "sub", "uid", "id"} {
					if s, ok := m[f].(string); ok && s != "" {
						return s
					}
				}
			}
		}
	}
	for _, h := range []string{"X-User", "X-User-ID", "X-User-Id"} {
		if s := strings.TrimSpace(r.Header.Get(h)); s != "" {
			return s
		}
	}
	return ""
}

// isAdminRequest reports admin role via context (role/is_admin/admin) with
// X-Role header fallback so admin routes work behind the base middleware.
func isAdminRequest(r *http.Request) bool {
	if UserRoleFromRequest != nil && strings.EqualFold(UserRoleFromRequest(r), "admin") {
		return true
	}
	ctx := r.Context()
	for _, k := range []any{"role", "user_role"} {
		if v := ctx.Value(k); v != nil {
			if s, ok := v.(string); ok && strings.EqualFold(s, "admin") {
				return true
			}
		}
	}
	for _, k := range []any{"is_admin", "admin"} {
		if v := ctx.Value(k); v != nil {
			if b, ok := v.(bool); ok && b {
				return true
			}
		}
	}
	if v := ctx.Value("user"); v != nil {
		if m, ok := v.(map[string]any); ok {
			if s, ok := m["role"].(string); ok && strings.EqualFold(s, "admin") {
				return true
			}
		}
	}
	for _, h := range []string{"X-Role", "X-User-Role"} {
		if strings.EqualFold(strings.TrimSpace(r.Header.Get(h)), "admin") {
			return true
		}
	}
	return false
}

func requireDB(s *Store, w http.ResponseWriter) bool {
	if s == nil || s.Pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable", "database not configured")
		return false
	}
	return true
}

func ctxOrBg(r *http.Request) context.Context { return r.Context() }
