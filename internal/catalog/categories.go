package catalog

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func decodeJSON(r *http.Request, v any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v) == nil
}

// --- Categories ---

func (s *Store) listCategories(w http.ResponseWriter, r *http.Request) {
	if !requireDB(s, w) {
		return
	}
	rows, err := s.Pool.Query(ctxOrBg(r), `SELECT id, name, slug, COALESCE(description,''), created_at, updated_at FROM categories ORDER BY name`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to list categories")
		return
	}
	defer rows.Close()
	type cat struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	out := []cat{}
	for rows.Next() {
		var c cat
		var ca, ua any
		_ = ca
		_ = ua
		var created, updated string
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &created, &updated); err != nil {
			continue
		}
		c.CreatedAt, c.UpdatedAt = created, updated
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Store) createCategory(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	var in struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if !decodeJSON(r, &in) || strings.TrimSpace(in.Name) == "" {
		writeErr(w, http.StatusBadRequest, "invalid_body", "name is required")
		return
	}
	slug := in.Slug
	if slug == "" {
		slug = slugify(in.Name)
	}
	var id, created, updated string
	err := s.Pool.QueryRow(ctxOrBg(r),
		`INSERT INTO categories(name, slug, description) VALUES($1,$2,$3) RETURNING id::text, created_at::text, updated_at::text`,
		strings.TrimSpace(in.Name), slug, in.Description).Scan(&id, &created, &updated)
	if err != nil {
		writeErr(w, http.StatusConflict, "conflict", "category slug already exists")
		return
	}
	w.Header().Set("Location", "/api/v1/categories/"+slug)
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{
		"id": id, "name": strings.TrimSpace(in.Name), "slug": slug,
		"description": in.Description, "created_at": created, "updated_at": updated,
	}})
}

func (s *Store) updateCategory(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	var in struct {
		Name        *string `json:"name"`
		Slug        *string `json:"slug"`
		Description *string `json:"description"`
	}
	if !decodeJSON(r, &in) {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON")
		return
	}
	var cid, name, slug, desc, created, updated string
	err := s.Pool.QueryRow(ctxOrBg(r),
		`UPDATE categories SET
			name = COALESCE(NULLIF($2,''), name),
			slug = COALESCE(NULLIF($3,''), slug),
			description = COALESCE($4, description),
			updated_at = now()
		WHERE id::text = $1
		RETURNING id::text, name, slug, COALESCE(description,''), created_at::text, updated_at::text`,
		id, strOr(in.Name), strOr(in.Slug), strOrNil(in.Description)).Scan(&cid, &name, &slug, &desc, &created, &updated)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "category not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"id": cid, "name": name, "slug": slug, "description": desc,
		"created_at": created, "updated_at": updated,
	}})
}

func (s *Store) deleteCategory(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	ct, err := s.Pool.Exec(ctxOrBg(r), `DELETE FROM categories WHERE id::text = $1`, id)
	if err != nil || ct.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "not_found", "category not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
