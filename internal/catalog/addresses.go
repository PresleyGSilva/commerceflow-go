package catalog

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type addressRow struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Label     string `json:"label,omitempty"`
	Street    string `json:"street"`
	City      string `json:"city"`
	State     string `json:"state,omitempty"`
	Zip       string `json:"zip,omitempty"`
	Country   string `json:"country"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

const addressSelect = `SELECT id::text, user_id::text, COALESCE(label,''), street, city,
	COALESCE(state,''), COALESCE(zip,''), COALESCE(country,'BR'), created_at::text, updated_at::text
	FROM addresses`

func (s *Store) requireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	uid := userFromRequest(r)
	if uid == "" {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return "", false
	}
	return uid, true
}

func (s *Store) listAddresses(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.requireUser(w, r)
	if !ok || !requireDB(s, w) {
		return
	}
	rows, err := s.Pool.Query(ctxOrBg(r), addressSelect+` WHERE user_id::text = $1 ORDER BY created_at DESC`, uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to list addresses")
		return
	}
	defer rows.Close()
	out := []addressRow{}
	for rows.Next() {
		var a addressRow
		if err := rows.Scan(&a.ID, &a.UserID, &a.Label, &a.Street, &a.City, &a.State, &a.Zip, &a.Country, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		out = append(out, a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Store) createAddress(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.requireUser(w, r)
	if !ok || !requireDB(s, w) {
		return
	}
	var in struct {
		Label   string `json:"label"`
		Street  string `json:"street"`
		City    string `json:"city"`
		State   string `json:"state"`
		Zip     string `json:"zip"`
		Country string `json:"country"`
	}
	if !decodeJSON(r, &in) || strings.TrimSpace(in.Street) == "" || strings.TrimSpace(in.City) == "" {
		writeErr(w, http.StatusBadRequest, "invalid_body", "street and city are required")
		return
	}
	country := in.Country
	if country == "" {
		country = "BR"
	}
	var a addressRow
	err := s.Pool.QueryRow(ctxOrBg(r),
		`INSERT INTO addresses(user_id, label, street, city, state, zip, country)
		 VALUES($1::uuid, $2, $3, $4, $5, $6, $7)
		 RETURNING id::text, user_id::text, COALESCE(label,''), street, city, COALESCE(state,''), COALESCE(zip,''), COALESCE(country,'BR'), created_at::text, updated_at::text`,
		uid, in.Label, strings.TrimSpace(in.Street), strings.TrimSpace(in.City), in.State, in.Zip, country).Scan(
		&a.ID, &a.UserID, &a.Label, &a.Street, &a.City, &a.State, &a.Zip, &a.Country, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "failed to create address")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": a})
}

func (s *Store) updateAddress(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.requireUser(w, r)
	if !ok || !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	var in map[string]any
	if !decodeJSON(r, &in) {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON")
		return
	}
	label, _ := in["label"]
	street, _ := in["street"]
	city, _ := in["city"]
	state, _ := in["state"]
	zip, _ := in["zip"]
	country, _ := in["country"]
	var a addressRow
	err := s.Pool.QueryRow(ctxOrBg(r),
		`UPDATE addresses SET
			label = CASE WHEN $3 IS NULL THEN label ELSE $3::text END,
			street = CASE WHEN $4 IS NULL OR $4 = '' THEN street ELSE $4::text END,
			city = CASE WHEN $5 IS NULL OR $5 = '' THEN city ELSE $5::text END,
			state = CASE WHEN $6 IS NULL THEN state ELSE $6::text END,
			zip = CASE WHEN $7 IS NULL THEN zip ELSE $7::text END,
			country = CASE WHEN $8 IS NULL OR $8 = '' THEN country ELSE $8::text END,
			updated_at = now()
		WHERE id::text = $1 AND user_id::text = $2
		RETURNING id::text, user_id::text, COALESCE(label,''), street, city, COALESCE(state,''), COALESCE(zip,''), COALESCE(country,'BR'), created_at::text, updated_at::text`,
		id, uid, nilIf(label), nilIf(street), nilIf(city), nilIf(state), nilIf(zip), nilIf(country),
	).Scan(&a.ID, &a.UserID, &a.Label, &a.Street, &a.City, &a.State, &a.Zip, &a.Country, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "address not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": a})
}

func nilIf(v any) any {
	if s, ok := v.(string); ok && s == "" {
		return ""
	}
	return v
}

func (s *Store) deleteAddress(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.requireUser(w, r)
	if !ok || !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	ct, err := s.Pool.Exec(ctxOrBg(r), `DELETE FROM addresses WHERE id::text = $1 AND user_id::text = $2`, id, uid)
	if err != nil || ct.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "not_found", "address not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
