package catalog

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type couponRow struct {
	ID               string  `json:"id"`
	Code             string  `json:"code"`
	Type             string  `json:"type"`
	Value            int     `json:"value"`
	MinSubtotalCents int     `json:"min_subtotal_cents"`
	Active           bool    `json:"active"`
	ExpiresAt        *string `json:"expires_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

const couponSelect = `SELECT id::text, code, type, value, min_subtotal_cents, active,
	expires_at::text, created_at::text, updated_at::text FROM coupons`

func scanCoupon(rows interface {
	Scan(...any) error
}, nullOK bool) (couponRow, Coupon, bool) {
	var c couponRow
	var expiresNs *string
	_ = nullOK
	var expAny *string
	err := rows.Scan(&c.ID, &c.Code, &c.Type, &c.Value, &c.MinSubtotalCents, &c.Active, &expAny, &c.CreatedAt, &c.UpdatedAt)
	_ = expiresNs
	if err != nil {
		return c, Coupon{}, false
	}
	if expAny != nil {
		c.ExpiresAt = expAny
	}
	var exp *time.Time
	if expAny != nil && *expAny != "" {
		if t, err := time.Parse(time.RFC3339, *expAny); err == nil {
			exp = &t
		} else if t, err := time.Parse("2006-01-02 15:04:05-07", *expAny); err == nil {
			exp = &t
		}
	}
	cp := Coupon{Code: c.Code, Type: c.Type, Value: c.Value,
		MinSubtotalCents: c.MinSubtotalCents, Active: c.Active, ExpiresAt: exp}
	return c, cp, true
}

// GET /api/v1/coupons/validate?code=&subtotal=
func (s *Store) validateCoupon(w http.ResponseWriter, r *http.Request) {
	if !requireDB(s, w) {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code")))
	subtotal, _ := strconv.Atoi(r.URL.Query().Get("subtotal"))
	if code == "" {
		writeErr(w, http.StatusBadRequest, "invalid_body", "code is required")
		return
	}
	var c couponRow
	var expStr *string
	err := s.Pool.QueryRow(ctxOrBg(r), couponSelect+` WHERE code = $1`, code).Scan(
		&c.ID, &c.Code, &c.Type, &c.Value, &c.MinSubtotalCents, &c.Active, &expStr, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "coupon not found")
		return
	}
	if expStr != nil && *expStr != "" {
		c.ExpiresAt = expStr
	}
	var exp *time.Time
	if expStr != nil && *expStr != "" {
		if t, err := time.Parse(time.RFC3339, *expStr); err == nil {
			exp = &t
		} else if t, err := time.Parse("2006-01-02 15:04:05-07", *expStr); err == nil {
			exp = &t
		} else if t, err := time.Parse("2006-01-02 15:04:05.999999-07", *expStr); err == nil {
			exp = &t
		}
	}
	now := time.Now()
	discount := CalcDiscount(subtotal, Coupon{
		Code: c.Code, Type: c.Type, Value: c.Value,
		MinSubtotalCents: c.MinSubtotalCents, Active: c.Active, ExpiresAt: exp,
	}, now)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"code": c.Code, "type": c.Type, "value": c.Value,
		"min_subtotal_cents": c.MinSubtotalCents, "active": c.Active,
		"expires_at": c.ExpiresAt, "subtotal_cents": subtotal,
		"discount_cents": discount, "valid": discount > 0,
	}})
}

func (s *Store) listCoupons(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	rows, err := s.Pool.Query(ctxOrBg(r), couponSelect+` ORDER BY created_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to list coupons")
		return
	}
	defer rows.Close()
	out := []couponRow{}
	for rows.Next() {
		var c couponRow
		var expStr *string
		if err := rows.Scan(&c.ID, &c.Code, &c.Type, &c.Value, &c.MinSubtotalCents, &c.Active, &expStr, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		if expStr != nil && *expStr != "" {
			c.ExpiresAt = expStr
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Store) createCoupon(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	var in struct {
		Code             string  `json:"code"`
		Type             string  `json:"type"`
		Value            int     `json:"value"`
		MinSubtotalCents int     `json:"min_subtotal_cents"`
		Active           *bool   `json:"active"`
		ExpiresAt        *string `json:"expires_at"`
	}
	if !decodeJSON(r, &in) || strings.TrimSpace(in.Code) == "" {
		writeErr(w, http.StatusBadRequest, "invalid_body", "code and type are required")
		return
	}
	if msg := ValidateCouponInput(in.Type, in.Value, in.MinSubtotalCents); msg != "" {
		writeErr(w, http.StatusBadRequest, "invalid_body", msg)
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	var c couponRow
	var expStr *string
	err := s.Pool.QueryRow(ctxOrBg(r),
		`INSERT INTO coupons(code, type, value, min_subtotal_cents, active, expires_at)
		 VALUES(UPPER($1),$2,$3,$4,$5,NULLIF($6,'')::timestamptz)
		 RETURNING id::text, code, type, value, min_subtotal_cents, active, expires_at::text, created_at::text, updated_at::text`,
		strings.TrimSpace(in.Code), in.Type, in.Value, in.MinSubtotalCents, active, strOr(in.ExpiresAt),
	).Scan(&c.ID, &c.Code, &c.Type, &c.Value, &c.MinSubtotalCents, &c.Active, &expStr, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusConflict, "conflict", "coupon code already exists")
		return
	}
	if expStr != nil && *expStr != "" {
		c.ExpiresAt = expStr
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": c})
}

func (s *Store) updateCoupon(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	var cur struct {
		Type  string
		Value int
		Min   int
	}
	if err := s.Pool.QueryRow(ctxOrBg(r),
		`SELECT type, value, min_subtotal_cents FROM coupons WHERE id::text = $1 OR code = UPPER($1)`, id).Scan(&cur.Type, &cur.Value, &cur.Min); err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "coupon not found")
		return
	}
	var in map[string]any
	if !decodeJSON(r, &in) {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON")
		return
	}
	typ := cur.Type
	if v, ok := in["type"].(string); ok && v != "" {
		typ = v
	}
	val := cur.Value
	if v, ok := in["value"].(float64); ok {
		val = int(v)
	}
	min := cur.Min
	if v, ok := in["min_subtotal_cents"].(float64); ok {
		min = int(v)
	}
	if msg := ValidateCouponInput(typ, val, min); msg != "" {
		writeErr(w, http.StatusBadRequest, "invalid_body", msg)
		return
	}
	code, _ := in["code"].(string)
	active, hasActive := in["active"]
	exp, hasExp := in["expires_at"]
	var c couponRow
	var expStr *string
	err := s.Pool.QueryRow(ctxOrBg(r),
		`UPDATE coupons SET
			code = CASE WHEN $2 = '' THEN code ELSE UPPER($2) END,
			type = $3, value = $4, min_subtotal_cents = $5,
			active = CASE WHEN $6 IS NULL THEN active ELSE $6::boolean END,
			expires_at = CASE WHEN $7 IS NULL THEN expires_at WHEN $7 = '' THEN NULL ELSE $7::timestamptz END,
			updated_at = now()
		WHERE id::text = $1 OR code = UPPER($1)
		RETURNING id::text, code, type, value, min_subtotal_cents, active, expires_at::text, created_at::text, updated_at::text`,
		id, code, typ, val, min, nilIfMissing(active, hasActive), nilIfMissing(exp, hasExp),
	).Scan(&c.ID, &c.Code, &c.Type, &c.Value, &c.MinSubtotalCents, &c.Active, &expStr, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "coupon not found")
		return
	}
	if expStr != nil && *expStr != "" {
		c.ExpiresAt = expStr
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

func (s *Store) deleteCoupon(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	ct, err := s.Pool.Exec(ctxOrBg(r), `DELETE FROM coupons WHERE id::text = $1 OR code = UPPER($1)`, id)
	if err != nil || ct.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "not_found", "coupon not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
