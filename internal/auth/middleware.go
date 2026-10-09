package auth

import (
	"context"
	"net/http"
	"strings"

	httpx "github.com/PresleyGSilva/commerceflow-go/internal/httpx"
)

type ctxKey string

const (
	ctxUserID ctxKey = "user_id"
	ctxEmail  ctxKey = "email"
	ctxRole   ctxKey = "role"
	ctxOrgID  ctxKey = "org_id"
)

func UserID(r *http.Request) string  { return val(r, ctxUserID) }
func UserRole(r *http.Request) string { return val(r, ctxRole) }
func OrgID(r *http.Request) string {
	// Header takes precedence (tenant switching), fallback to claim.
	if h := r.Header.Get("X-Org-ID"); h != "" {
		return h
	}
	return val(r, ctxOrgID)
}

func val(r *http.Request, k ctxKey) string {
	if v, ok := r.Context().Value(k).(string); ok {
		return v
	}
	return ""
}

// Middleware validates Bearer access token.
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			c, err := Parse(secret, strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid token")
				return
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxUserID, c.Subject)
			ctx = context.WithValue(ctx, ctxEmail, c.Email)
			ctx = context.WithValue(ctx, ctxRole, c.Role)
			ctx = context.WithValue(ctx, ctxOrgID, c.OrgID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole blocks unless the caller's role matches one of the given roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allow := map[string]bool{}
	for _, r := range roles {
		allow[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allow[UserRole(r)] {
				httpx.Error(w, http.StatusForbidden, "forbidden", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
