package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	httpx "github.com/PresleyGSilva/commerceflow-go/internal/httpx"
)

type Handler struct {
	DB          *pgxpool.Pool
	JWTSecret   string
	RefreshTTLH int
}

type OrgInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func (h *Handler) routes(mux *http.ServeMux) {}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Email == "" || len(in.Password) < 8 || strings.TrimSpace(in.Name) == "" {
		httpx.Error(w, http.StatusBadRequest, "validation_error", "email, name and password (min 8 chars) are required")
		return
	}
	hash, err := HashPassword(in.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "could not hash password")
		return
	}
	ctx := r.Context()
	var id, role string
	err = h.DB.QueryRow(ctx, `INSERT INTO users(email,password_hash,name,role) VALUES($1,$2,$3,'customer') RETURNING id, role`, in.Email, hash, strings.TrimSpace(in.Name)).Scan(&id, &role)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			httpx.Error(w, http.StatusConflict, "conflict", "email already registered")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "could not create user")
		return
	}
	// Attach to default org if present.
	var orgID, orgName string
	_ = h.DB.QueryRow(ctx, `SELECT id, name FROM orgs ORDER BY created_at LIMIT 1`).Scan(&orgID, &orgName)
	orgs := []OrgInfo{}
	if orgID != "" {
		_, _ = h.DB.Exec(ctx, `INSERT INTO org_members(org_id,user_id,role) VALUES($1,$2,'member') ON CONFLICT DO NOTHING`, orgID, id)
		orgs = append(orgs, OrgInfo{ID: orgID, Name: orgName, Role: "member"})
	}
	access, _ := SignAccess(h.JWTSecret, id, in.Email, role, firstOrgID(orgs))
	httpx.JSON(w, http.StatusCreated, map[string]any{"data": map[string]any{
		"access_token": access,
		"user":         map[string]any{"id": id, "email": in.Email, "name": strings.TrimSpace(in.Name), "role": role},
		"orgs":         orgs,
	}})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	ctx := r.Context()
	var id, hash, name, role string
	err := h.DB.QueryRow(ctx, `SELECT id, password_hash, name, role FROM users WHERE email=$1`, in.Email).Scan(&id, &hash, &name, &role)
	if err != nil || !CheckPassword(hash, in.Password) {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}
	rows, _ := h.DB.Query(ctx, `SELECT m.org_id, o.name, m.role FROM org_members m JOIN orgs o ON o.id=m.org_id WHERE m.user_id=$1`, id)
	orgs := []OrgInfo{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var o OrgInfo
			if err := rows.Scan(&o.ID, &o.Name, &o.Role); err == nil {
				orgs = append(orgs, o)
			}
		}
	}
	if orgs == nil {
		orgs = []OrgInfo{}
	}
	access, _ := SignAccess(h.JWTSecret, id, in.Email, role, firstOrgID(orgs))
	refresh, _ := SignRefresh(h.JWTSecret, id, h.RefreshTTLH)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Duration(h.RefreshTTLH) * time.Hour),
	})
	httpx.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"access_token": access,
		"user":         map[string]any{"id": id, "email": in.Email, "name": name, "role": role},
		"orgs":         orgs,
	}})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing refresh token")
		return
	}
	claims, err := Parse(h.JWTSecret, c.Value)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token")
		return
	}
	ctx := r.Context()
	var email, role string
	if err := h.DB.QueryRow(ctx, `SELECT email, role FROM users WHERE id=$1`, claims.Subject).Scan(&email, &role); err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "user not found")
		return
	}
	var orgID string
	_ = h.DB.QueryRow(ctx, `SELECT org_id FROM org_members WHERE user_id=$1 ORDER BY created_at LIMIT 1`, claims.Subject).Scan(&orgID)
	access, _ := SignAccess(h.JWTSecret, claims.Subject, email, role, orgID)
	httpx.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"access_token": access}})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "", Path: "/api/v1/auth", HttpOnly: true, MaxAge: -1})
	httpx.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"ok": true}})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uid := UserID(r)
	ctx := r.Context()
	var email, name, role string
	if err := h.DB.QueryRow(ctx, `SELECT email, name, role FROM users WHERE id=$1`, uid).Scan(&email, &name, &role); err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	rows, _ := h.DB.Query(ctx, `SELECT m.org_id, o.name, m.role FROM org_members m JOIN orgs o ON o.id=m.org_id WHERE m.user_id=$1`, uid)
	orgs := []OrgInfo{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var o OrgInfo
			if err := rows.Scan(&o.ID, &o.Name, &o.Role); err == nil {
				orgs = append(orgs, o)
			}
		}
	}
	if orgs == nil {
		orgs = []OrgInfo{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"user": map[string]any{"id": uid, "email": email, "name": name, "role": role},
		"orgs": orgs,
	}})
}

func firstOrgID(orgs []OrgInfo) string {
	if len(orgs) > 0 {
		return orgs[0].ID
	}
	return ""
}
