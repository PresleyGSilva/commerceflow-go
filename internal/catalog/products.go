package catalog

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type productRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	PriceCents  int    `json:"price_cents"`
	StockQty    int    `json:"stock_qty"`
	CategoryID  string `json:"category_id,omitempty"`
	Category    string `json:"category,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

const productSelect = `SELECT p.id::text, p.name, p.slug, COALESCE(p.description,''), p.price_cents,
	p.stock_qty, COALESCE(p.category_id::text,''), COALESCE(c.slug,''), COALESCE(p.image_url,''),
	p.active, p.created_at::text, p.updated_at::text
	FROM products p LEFT JOIN categories c ON c.id = p.category_id`

func (s *Store) listProducts(w http.ResponseWriter, r *http.Request) {
	if !requireDB(s, w) {
		return
	}
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("q"))
	category := strings.TrimSpace(q.Get("category"))
	minP, _ := strconv.Atoi(q.Get("min_price"))
	maxP, _ := strconv.Atoi(q.Get("max_price"))
	sort := q.Get("sort")
	if sort == "" {
		sort = q.Get("order")
	}
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	where := "WHERE p.active = true"
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where += fmt.Sprintf(" AND "+cond, len(args))
	}
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		where += fmt.Sprintf(" AND (LOWER(p.name) LIKE $%d OR LOWER(COALESCE(p.description,'')) LIKE $%d)", len(args), len(args))
	}
	if category != "" {
		args = append(args, category)
		where += fmt.Sprintf(" AND (c.slug = $%d OR p.category_id::text = $%d)", len(args), len(args))
	}
	if q.Get("min_price") != "" {
		add("p.price_cents >= $%d", minP)
	}
	if q.Get("max_price") != "" {
		add("p.price_cents <= $%d", maxP)
	}

	orderBy := "p.created_at DESC"
	switch sort {
	case "price_asc":
		orderBy = "p.price_cents ASC"
	case "price_desc":
		orderBy = "p.price_cents DESC"
	case "newest":
		orderBy = "p.created_at DESC"
	case "name_asc":
		orderBy = "p.name ASC"
	}

	var total int
	if err := s.Pool.QueryRow(ctxOrBg(r),
		`SELECT COUNT(*) FROM products p LEFT JOIN categories c ON c.id = p.category_id `+where, args...).Scan(&total); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to count products")
		return
	}
	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.Pool.Query(ctxOrBg(r), productSelect+` `+where+
		fmt.Sprintf(` ORDER BY %s LIMIT $%d OFFSET $%d`, orderBy, len(args)-1, len(args)), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to list products")
		return
	}
	defer rows.Close()
	out := []productRow{}
	for rows.Next() {
		var p productRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.PriceCents,
			&p.StockQty, &p.CategoryID, &p.Category, &p.ImageURL, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out,
		"meta": map[string]any{"page": page, "per_page": perPage, "total": total}})
}

func (s *Store) getProduct(w http.ResponseWriter, r *http.Request) {
	if !requireDB(s, w) {
		return
	}
	slug := chi.URLParam(r, "slug")
	var p productRow
	err := s.Pool.QueryRow(ctxOrBg(r), productSelect+` WHERE p.slug = $1`, slug).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.PriceCents,
		&p.StockQty, &p.CategoryID, &p.Category, &p.ImageURL, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "product not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

func (s *Store) createProduct(w http.ResponseWriter, r *http.Request) {
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
		PriceCents  int    `json:"price_cents"`
		StockQty    int    `json:"stock_qty"`
		CategoryID  string `json:"category_id"`
		Category    string `json:"category"`
		ImageURL    string `json:"image_url"`
		Active      *bool  `json:"active"`
	}
	if !decodeJSON(r, &in) || strings.TrimSpace(in.Name) == "" {
		writeErr(w, http.StatusBadRequest, "invalid_body", "name is required")
		return
	}
	if in.PriceCents < 0 || in.StockQty < 0 {
		writeErr(w, http.StatusBadRequest, "invalid_body", "price_cents and stock_qty must be >= 0")
		return
	}
	slug := in.Slug
	if slug == "" {
		slug = slugify(in.Name)
	}
	catID := in.CategoryID
	if catID == "" && in.Category != "" {
		_ = s.Pool.QueryRow(ctxOrBg(r), `SELECT id::text FROM categories WHERE slug = $1`, in.Category).Scan(&catID)
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	var p productRow
	err := s.Pool.QueryRow(ctxOrBg(r),
		`INSERT INTO products(name, slug, description, price_cents, stock_qty, category_id, image_url, active)
		 VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::uuid,$7,$8)
		 RETURNING id::text, name, slug, COALESCE(description,''), price_cents, stock_qty,
			COALESCE(category_id::text,''), '', COALESCE(image_url,''), active, created_at::text, updated_at::text`,
		strings.TrimSpace(in.Name), slug, in.Description, in.PriceCents, in.StockQty, catID, in.ImageURL, active,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.PriceCents, &p.StockQty,
		&p.CategoryID, &p.Category, &p.ImageURL, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusConflict, "conflict", "product slug already exists or invalid category")
		return
	}
	w.Header().Set("Location", "/api/v1/products/"+p.Slug)
	writeJSON(w, http.StatusCreated, map[string]any{"data": p})
}

func (s *Store) updateProduct(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	var in map[string]any
	if !decodeJSON(r, &in) {
		writeErr(w, http.StatusBadRequest, "invalid_body", "invalid JSON")
		return
	}
	if v, ok := in["price_cents"]; ok {
		if f, ok := v.(float64); !ok || f < 0 {
			writeErr(w, http.StatusBadRequest, "invalid_body", "price_cents must be >= 0")
			return
		}
	}
	if v, ok := in["stock_qty"]; ok {
		if f, ok := v.(float64); !ok || f < 0 {
			writeErr(w, http.StatusBadRequest, "invalid_body", "stock_qty must be >= 0")
			return
		}
	}
	name, _ := in["name"].(string)
	slug, _ := in["slug"].(string)
	desc, hasDesc := in["description"]
	price, hasPrice := in["price_cents"]
	stock, hasStock := in["stock_qty"]
	img, hasImg := in["image_url"]
	active, hasActive := in["active"]
	cat, hasCat := in["category_id"]
	var p productRow
	err := s.Pool.QueryRow(ctxOrBg(r),
		`UPDATE products SET
			name = CASE WHEN $2 = '' THEN name ELSE $2 END,
			slug = CASE WHEN $3 = '' THEN slug ELSE $3 END,
			description = CASE WHEN $4 IS NULL THEN description ELSE $4::text END,
			price_cents = CASE WHEN $5 IS NULL THEN price_cents ELSE $5::int END,
			stock_qty = CASE WHEN $6 IS NULL THEN stock_qty ELSE $6::int END,
			category_id = CASE WHEN $7 IS NULL OR $7 = '' THEN category_id ELSE $7::uuid END,
			image_url = CASE WHEN $8 IS NULL THEN image_url ELSE $8::text END,
			active = CASE WHEN $9 IS NULL THEN active ELSE $9::boolean END,
			updated_at = now()
		WHERE id::text = $1 OR slug = $1
		RETURNING id::text, name, slug, COALESCE(description,''), price_cents, stock_qty,
			COALESCE(category_id::text,''), '', COALESCE(image_url,''), active, created_at::text, updated_at::text`,
		id, name, slug, nilIfMissing(desc, hasDesc), nilIfMissing(price, hasPrice),
		nilIfMissing(stock, hasStock), nilIfMissing(catStr(cat, hasCat), hasCat),
		nilIfMissing(img, hasImg), nilIfMissing(active, hasActive),
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.PriceCents, &p.StockQty,
		&p.CategoryID, &p.Category, &p.ImageURL, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "product not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

func catStr(v any, ok bool) any {
	if !ok {
		return nil
	}
	s, _ := v.(string)
	return s
}

func nilIfMissing(v any, ok bool) any {
	if !ok {
		return nil
	}
	return v
}

func (s *Store) deleteProduct(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		writeErr(w, http.StatusForbidden, "forbidden", "admin only")
		return
	}
	if !requireDB(s, w) {
		return
	}
	id := chi.URLParam(r, "id")
	ct, err := s.Pool.Exec(ctxOrBg(r), `DELETE FROM products WHERE id::text = $1 OR slug = $1`, id)
	if err != nil || ct.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "not_found", "product not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
