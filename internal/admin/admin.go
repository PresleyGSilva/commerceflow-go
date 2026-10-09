// Package admin serves the admin dashboard aggregates and reports.
//
// All routes must be mounted behind auth.Middleware + RequireRole("admin")
// (see internal/http MountAdmin). Money is always integer cents.
package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LowStockThreshold flags products with at most this many units.
const LowStockThreshold = 5

// Handler serves admin endpoints backed by pool.
type Handler struct {
	DB *pgxpool.Pool
}

// New returns a Handler backed by db (may be nil only in tests).
func New(db *pgxpool.Pool) *Handler { return &Handler{DB: db} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeData(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h == nil || h.DB == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable", "database not configured")
		return false
	}
	return true
}

type lowStockProduct struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	StockQty   int    `json:"stock_qty"`
	PriceCents int    `json:"price_cents"`
}

type topProduct struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Qty          int64  `json:"qty"`
	RevenueCents int64  `json:"revenue_cents"`
}

// Dashboard aggregates revenue, order counts, low stock and top sellers.
//
// GET /api/v1/admin/dashboard
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	ctx := r.Context()

	var revenueCents, ordersCount, paidCount int64
	if err := h.DB.QueryRow(ctx, `SELECT COALESCE(SUM(total_cents),0) FROM orders WHERE status='paid'`).Scan(&revenueCents); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to sum revenue")
		return
	}
	if err := h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&ordersCount); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to count orders")
		return
	}
	if err := h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE status='paid'`).Scan(&paidCount); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to count paid orders")
		return
	}

	low := []lowStockProduct{}
	rows, err := h.DB.Query(ctx, `SELECT id::text, name, slug, stock_qty, price_cents FROM products WHERE stock_qty <= $1 ORDER BY stock_qty ASC, name ASC LIMIT 10`, LowStockThreshold)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to list low stock")
		return
	}
	for rows.Next() {
		var p lowStockProduct
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.StockQty, &p.PriceCents); err != nil {
			rows.Close()
			writeErr(w, http.StatusInternalServerError, "internal", "failed to scan product")
			return
		}
		low = append(low, p)
	}
	rows.Close()

	top := []topProduct{}
	trows, err := h.DB.Query(ctx, `
		SELECT p.id::text, p.name, SUM(oi.qty)::bigint, SUM(oi.qty * oi.unit_price_cents)::bigint
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		JOIN products p ON p.id = oi.product_id
		WHERE o.status = 'paid'
		GROUP BY p.id, p.name
		ORDER BY SUM(oi.qty * oi.unit_price_cents) DESC
		LIMIT 5`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to rank top products")
		return
	}
	for trows.Next() {
		var t topProduct
		if err := trows.Scan(&t.ID, &t.Name, &t.Qty, &t.RevenueCents); err != nil {
			trows.Close()
			writeErr(w, http.StatusInternalServerError, "internal", "failed to scan top product")
			return
		}
		top = append(top, t)
	}
	trows.Close()

	writeData(w, map[string]any{
		"revenue_cents": revenueCents,
		"orders_count":  ordersCount,
		"paid_count":    paidCount,
		"low_stock":     low,
		"top_products":  top,
	})
}

type salesPoint struct {
	Date         string `json:"date"`
	OrdersCount  int64  `json:"orders_count"`
	RevenueCents int64  `json:"revenue_cents"`
}

func parseDateParam(v string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// SalesReport sums paid orders per day in [from, to].
//
// GET /api/v1/admin/reports/sales?from=YYYY-MM-DD&to=YYYY-MM-DD&group_by=day
func (h *Handler) SalesReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	q := r.URL.Query()
	if g := q.Get("group_by"); g != "" && g != "day" {
		writeErr(w, http.StatusBadRequest, "invalid_param", "group_by must be day")
		return
	}
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -29)
	if v := q.Get("from"); v != "" {
		t, ok := parseDateParam(v)
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid_param", "from must be YYYY-MM-DD")
			return
		}
		from = t
	}
	if v := q.Get("to"); v != "" {
		t, ok := parseDateParam(v)
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid_param", "to must be YYYY-MM-DD")
			return
		}
		to = t
	}
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	if fromDay.After(toDay) {
		writeErr(w, http.StatusBadRequest, "invalid_param", "from must not be after to")
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT (date_trunc('day', created_at)::date)::text, COUNT(*), COALESCE(SUM(total_cents),0)
		FROM orders
		WHERE status = 'paid' AND created_at >= $1 AND created_at < $2
		GROUP BY 1 ORDER BY 1`, fromDay, toDay.AddDate(0, 0, 1))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to aggregate sales")
		return
	}
	byDay := map[string][2]int64{}
	for rows.Next() {
		var day string
		var n, sum int64
		if err := rows.Scan(&day, &n, &sum); err != nil {
			rows.Close()
			writeErr(w, http.StatusInternalServerError, "internal", "failed to scan sales")
			return
		}
		byDay[day] = [2]int64{n, sum}
	}
	rows.Close()

	points := []salesPoint{}
	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		v := byDay[key]
		points = append(points, salesPoint{Date: key, OrdersCount: v[0], RevenueCents: v[1]})
	}

	writeData(w, map[string]any{
		"from":     fromDay.Format("2006-01-02"),
		"to":       toDay.Format("2006-01-02"),
		"group_by": "day",
		"points":   points,
	})
}

type stockItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	StockQty        int    `json:"stock_qty"`
	PriceCents      int    `json:"price_cents"`
	StockValueCents int64  `json:"stock_value_cents"`
	LowStock        bool   `json:"low_stock"`
}

// StockReport lists products with on-hand value in cents.
//
// GET /api/v1/admin/reports/stock
func (h *Handler) StockReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.DB.Query(r.Context(), `
		SELECT id::text, name, slug, stock_qty, price_cents, (stock_qty::bigint * price_cents::bigint)
		FROM products ORDER BY 6 DESC, name ASC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "failed to list stock")
		return
	}
	items := []stockItem{}
	var totalValue int64
	var lowCount int64
	for rows.Next() {
		var it stockItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Slug, &it.StockQty, &it.PriceCents, &it.StockValueCents); err != nil {
			rows.Close()
			writeErr(w, http.StatusInternalServerError, "internal", "failed to scan stock")
			return
		}
		it.LowStock = it.StockQty <= LowStockThreshold
		if it.LowStock {
			lowCount++
		}
		totalValue += it.StockValueCents
		items = append(items, it)
	}
	rows.Close()

	writeData(w, map[string]any{
		"items":             items,
		"total_value_cents": totalValue,
		"low_stock_count":   lowCount,
	})
}
