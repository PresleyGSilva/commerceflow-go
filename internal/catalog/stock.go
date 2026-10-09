package catalog

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// PATCH /api/v1/admin/products/:id/stock {delta, reason}
func (s *Store) adjustStock(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	var in struct {
		Delta  *int   `json:"delta"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(r, &in) || in.Delta == nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "delta is required")
		return
	}
	ctx := ctxOrBg(r)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)
	var stock int
	var pid string
	err = tx.QueryRow(ctx, `SELECT id::text, stock_qty FROM products WHERE id::text = $1 OR slug = $1 FOR UPDATE`, id).Scan(&pid, &stock)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "product not found")
		return
	}
	next := stock + *in.Delta
	if next < 0 {
		writeErr(w, http.StatusUnprocessableEntity, "insufficient_stock", "stock cannot go negative")
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE products SET stock_qty = $2, updated_at = now() WHERE id::text = $1`, pid, next); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to update stock")
		return
	}
	var movID, created string
	if err := tx.QueryRow(ctx,
		`INSERT INTO stock_movements(product_id, qty, reason) VALUES($1::uuid, $2, $3) RETURNING id::text, created_at::text`,
		pid, *in.Delta, in.Reason).Scan(&movID, &created); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to record movement")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to commit stock change")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"product_id": pid, "stock_qty": next,
		"movement": map[string]any{"id": movID, "delta": *in.Delta, "reason": in.Reason, "created_at": created},
	}})
}

// GET /api/v1/admin/stock/movements?page=&per_page=
func (s *Store) listMovements(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	product := q.Get("product_id")
	where := ""
	args := []any{}
	if product != "" {
		where = "WHERE m.product_id::text = $1 OR p.slug = $1"
		args = append(args, product)
	}
	var total int
	if err := s.Pool.QueryRow(ctxOrBg(r),
		`SELECT COUNT(*) FROM stock_movements m JOIN products p ON p.id = m.product_id `+where, args...).Scan(&total); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to count movements")
		return
	}
	args = append(args, perPage, (page-1)*perPage)
	n := len(args)
	rows, err := s.Pool.Query(ctxOrBg(r),
		`SELECT m.id::text, m.product_id::text, p.slug, p.name, m.qty, COALESCE(m.reason,''), m.created_at::text
		 FROM stock_movements m JOIN products p ON p.id = m.product_id `+where+
			` ORDER BY m.created_at DESC LIMIT $`+strconv.Itoa(n-1)+` OFFSET $`+strconv.Itoa(n), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to list movements")
		return
	}
	defer rows.Close()
	type mov struct {
		ID        string `json:"id"`
		ProductID string `json:"product_id"`
		Slug      string `json:"product_slug"`
		Name      string `json:"product_name"`
		Delta     int    `json:"delta"`
		Reason    string `json:"reason"`
		CreatedAt string `json:"created_at"`
	}
	out := []mov{}
	for rows.Next() {
		var m mov
		if err := rows.Scan(&m.ID, &m.ProductID, &m.Slug, &m.Name, &m.Delta, &m.Reason, &m.CreatedAt); err != nil {
			continue
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out,
		"meta": map[string]any{"page": page, "per_page": perPage, "total": total}})
}
