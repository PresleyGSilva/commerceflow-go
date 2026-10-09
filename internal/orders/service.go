package orders

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PresleyGSilva/commerceflow-go/internal/catalog"
	"github.com/PresleyGSilva/commerceflow-go/internal/payment"
)

// Totals is a price breakdown in integer cents. Money is NEVER float.
type Totals struct {
	SubtotalCents int `json:"subtotal_cents"`
	DiscountCents int `json:"discount_cents"`
	TotalCents    int `json:"total_cents"`
}

type productLock struct {
	ID         string
	Name       string
	PriceCents int
	StockQty   int
	Active     bool
}

// normalizeItems validates inputs and merges duplicated product ids.
func normalizeItems(items []ItemInput) ([]ItemInput, *APIError) {
	if len(items) == 0 {
		return nil, badRequest("empty_cart", "at least one item is required")
	}
	merged := map[string]int{}
	for _, it := range items {
		id := strings.TrimSpace(it.ProductID)
		if id == "" {
			return nil, badRequest("invalid_product", "product_id is required")
		}
		if it.Qty <= 0 {
			return nil, badRequest("invalid_qty", "qty must be > 0")
		}
		merged[id] += it.Qty
	}
	out := make([]ItemInput, 0, len(merged))
	for id, qty := range merged {
		out = append(out, ItemInput{ProductID: id, Qty: qty})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProductID < out[j].ProductID })
	return out, nil
}

// loadCoupon fetches a coupon row and maps it to catalog.Coupon.
// Unknown codes and inactive/expired coupons are 400 invalid_coupon;
// a below-minimum subtotal simply yields discount 0 (handled by CalcDiscount).
func loadCoupon(ctx context.Context, q dbQuerier, code string) (catalog.Coupon, *APIError) {
	var c catalog.Coupon
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return c, nil
	}
	var expires pgtype.Timestamptz
	err := q.QueryRow(ctx, `SELECT code, type, value, min_subtotal_cents, active, expires_at
		FROM coupons WHERE code = $1`, code).Scan(
		&c.Code, &c.Type, &c.Value, &c.MinSubtotalCents, &c.Active, &expires)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c, badRequest("invalid_coupon", "coupon not found")
		}
		return c, &APIError{Status: 500, Code: "internal", Message: "failed to load coupon"}
	}
	if expires.Valid {
		t := expires.Time
		c.ExpiresAt = &t
	}
	if !c.Active || (c.ExpiresAt != nil && time.Now().After(*c.ExpiresAt)) {
		return c, badRequest("invalid_coupon", "coupon is inactive or expired")
	}
	return c, nil
}

// dbQuerier abstracts *pgxpool.Pool and pgx.Tx for the queries this slice needs.
type dbQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// lockProducts selects the requested products ORDER BY id FOR UPDATE
// (stable order avoids deadlocks under concurrency).
func lockProducts(ctx context.Context, tx pgx.Tx, ids []string) (map[string]productLock, *APIError) {
	rows, err := tx.Query(ctx, `SELECT id::text, name, price_cents, stock_qty, active
		FROM products WHERE id::text = ANY($1) ORDER BY id FOR UPDATE`, ids)
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to lock products"}
	}
	defer rows.Close()
	found := map[string]productLock{}
	for rows.Next() {
		var p productLock
		if err := rows.Scan(&p.ID, &p.Name, &p.PriceCents, &p.StockQty, &p.Active); err != nil {
			return nil, &APIError{Status: 500, Code: "internal", Message: "failed to read products"}
		}
		found[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to read products"}
	}
	return found, nil
}

// readProducts is the non-locking variant used by preview.
func readProducts(ctx context.Context, q dbQuerier, ids []string) (map[string]productLock, *APIError) {
	rows, err := q.Query(ctx, `SELECT id::text, name, price_cents, stock_qty, active
		FROM products WHERE id::text = ANY($1)`, ids)
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to read products"}
	}
	defer rows.Close()
	found := map[string]productLock{}
	for rows.Next() {
		var p productLock
		if err := rows.Scan(&p.ID, &p.Name, &p.PriceCents, &p.StockQty, &p.Active); err != nil {
			return nil, &APIError{Status: 500, Code: "internal", Message: "failed to read products"}
		}
		found[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to read products"}
	}
	return found, nil
}

// Preview computes cart totals without persisting anything.
func (s *Store) Preview(ctx context.Context, items []ItemInput, couponCode string) (Totals, *APIError) {
	var t Totals
	if s.Pool == nil {
		return t, &APIError{Status: 503, Code: "unavailable", Message: "database not configured"}
	}
	norm, apiErr := normalizeItems(items)
	if apiErr != nil {
		return t, apiErr
	}
	coupon, apiErr := loadCoupon(ctx, s.Pool, couponCode)
	if apiErr != nil {
		return t, apiErr
	}
	ids := make([]string, 0, len(norm))
	for _, it := range norm {
		ids = append(ids, it.ProductID)
	}
	found, apiErr := readProducts(ctx, s.Pool, ids)
	if apiErr != nil {
		return t, apiErr
	}
	subtotal := 0
	for _, it := range norm {
		p, ok := found[it.ProductID]
		if !ok {
			return t, notFound("product not found: " + it.ProductID)
		}
		if !p.Active {
			return t, badRequest("product_unavailable", "product is not available: "+p.Name)
		}
		subtotal += p.PriceCents * it.Qty
	}
	discount := catalog.CalcDiscount(subtotal, coupon, time.Now())
	t = Totals{SubtotalCents: subtotal, DiscountCents: discount, TotalCents: subtotal - discount}
	return t, nil
}

// Checkout creates an order atomically. Idempotency-Key (idemKey) is required:
// repeating a key returns the original order with created=false, without
// touching stock. paymentMode is approve|decline|fail ("" defaults to approve).
// decline/fail persist the order as failed AND restore stock in the same
// transaction (logical rollback with compensating stock_movements).
func (s *Store) Checkout(ctx context.Context, userID, orgID string, items []ItemInput, addressID, couponCode, paymentMode, idemKey string) (*Order, bool, *APIError) {
	if s.Pool == nil {
		return nil, false, &APIError{Status: 503, Code: "unavailable", Message: "database not configured"}
	}
	if strings.TrimSpace(idemKey) == "" {
		return nil, false, badRequest("missing_idempotency_key", "Idempotency-Key header is required")
	}
	mode := payment.Normalize(paymentMode)
	if !payment.ValidMode(paymentMode) {
		return nil, false, badRequest("invalid_payment_mode", "payment_mode must be approve|decline|fail")
	}
	norm, apiErr := normalizeItems(items)
	if apiErr != nil {
		return nil, false, apiErr
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to begin transaction"}
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Idempotent replay: same (user, key) returns the original order.
	var existingID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM orders WHERE user_id::text = $1 AND idempotency_key = $2`,
		userID, idemKey).Scan(&existingID)
	if err == nil {
		o, apiErr := loadOrderTx(ctx, tx, existingID)
		if apiErr != nil {
			return nil, false, apiErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to commit"}
		}
		return o, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to check idempotency"}
	}

	if strings.TrimSpace(addressID) != "" {
		var one int
		err := tx.QueryRow(ctx, `SELECT 1 FROM addresses WHERE id::text = $1 AND user_id::text = $2`,
			strings.TrimSpace(addressID), userID).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, notFound("address not found")
		}
		if err != nil {
			return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to validate address"}
		}
	}

	coupon, apiErr := loadCoupon(ctx, tx, couponCode)
	if apiErr != nil {
		return nil, false, apiErr
	}

	ids := make([]string, 0, len(norm))
	for _, it := range norm {
		ids = append(ids, it.ProductID)
	}
	found, apiErr := lockProducts(ctx, tx, ids)
	if apiErr != nil {
		return nil, false, apiErr
	}
	subtotal := 0
	for _, it := range norm {
		p, ok := found[it.ProductID]
		if !ok {
			return nil, false, notFound("product not found: " + it.ProductID)
		}
		if !p.Active {
			return nil, false, badRequest("product_unavailable", "product is not available: "+p.Name)
		}
		if p.StockQty < it.Qty {
			return nil, false, badRequest("insufficient_stock",
				fmt.Sprintf("insufficient stock for %s: have %d, want %d", p.Name, p.StockQty, it.Qty))
		}
		subtotal += p.PriceCents * it.Qty
	}
	discount := catalog.CalcDiscount(subtotal, coupon, time.Now())
	total := subtotal - discount

	for _, it := range norm {
		if _, err := tx.Exec(ctx, `UPDATE products SET stock_qty = stock_qty - $2, updated_at = now()
			WHERE id::text = $1`, it.ProductID, it.Qty); err != nil {
			return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to decrement stock"}
		}
	}

	var orderID, createdAt string
	err = tx.QueryRow(ctx, `INSERT INTO orders(user_id, org_id, status, subtotal_cents, discount_cents,
		total_cents, coupon_code, idempotency_key, payment_mode, address_id, stock_reserved)
		VALUES($1::uuid, NULLIF($2,'')::uuid, 'pending', $3, $4, $5, $6, $7, $8, NULLIF($9,'')::uuid, true)
		RETURNING id::text, created_at::text`,
		userID, strings.TrimSpace(orgID), subtotal, discount, total,
		strings.ToUpper(strings.TrimSpace(couponCode)), idemKey, mode, strings.TrimSpace(addressID),
	).Scan(&orderID, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Lost a race with an identical key: fetch the winner.
			_ = tx.Rollback(ctx)
			o, apiErr := s.loadOrderByKey(ctx, userID, idemKey)
			if apiErr != nil {
				return nil, false, apiErr
			}
			return o, false, nil
		}
		return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to create order"}
	}

	for _, it := range norm {
		p := found[it.ProductID]
		if _, err := tx.Exec(ctx, `INSERT INTO order_items(order_id, product_id, product_name, qty, unit_price_cents)
			VALUES($1::uuid, $2::uuid, $3, $4, $5)`, orderID, p.ID, p.Name, it.Qty, p.PriceCents); err != nil {
			return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to create order items"}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stock_movements(product_id, order_id, qty, reason)
			VALUES($1::uuid, $2::uuid, $3, $4)`, p.ID, orderID, -it.Qty, "order:"+orderID); err != nil {
			return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to record stock movement"}
		}
	}

	approved, authCode, simErr := payment.Simulate(mode)
	finalStatus := StatusPaid
	reserved := true
	if simErr != nil || !approved {
		// Logical rollback: restore stock in the same tx, keep audit trail.
		for _, it := range norm {
			p := found[it.ProductID]
			if _, err := tx.Exec(ctx, `UPDATE products SET stock_qty = stock_qty + $2, updated_at = now()
				WHERE id::text = $1`, it.ProductID, it.Qty); err != nil {
				return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to restore stock"}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO stock_movements(product_id, order_id, qty, reason)
				VALUES($1::uuid, $2::uuid, $3, $4)`, p.ID, orderID, it.Qty, "rollback:"+orderID); err != nil {
				return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to record rollback"}
			}
		}
		finalStatus = StatusFailed
		authCode = ""
		reserved = false
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET status = $2, payment_auth_code = $3, stock_reserved = $4 WHERE id::text = $1`,
		orderID, finalStatus, authCode, reserved); err != nil {
		return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to finalize order"}
	}

	if err := tx.Commit(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			o, apiErr := s.loadOrderByKey(ctx, userID, idemKey)
			if apiErr != nil {
				return nil, false, apiErr
			}
			return o, false, nil
		}
		return nil, false, &APIError{Status: 500, Code: "internal", Message: "failed to commit order"}
	}

	o, apiErr := s.loadOrderByKey(ctx, userID, idemKey)
	if apiErr != nil {
		return nil, false, apiErr
	}
	return o, true, nil
}

// loadOrderByKey fetches an order (with items) by (user, idempotency key).
func (s *Store) loadOrderByKey(ctx context.Context, userID, idemKey string) (*Order, *APIError) {
	var id string
	err := s.Pool.QueryRow(ctx, `SELECT id::text FROM orders WHERE user_id::text = $1 AND idempotency_key = $2`,
		userID, idemKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("order not found")
	}
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to load order"}
	}
	return s.loadOrder(ctx, id)
}

// loadOrder fetches an order (with items) by id.
func (s *Store) loadOrder(ctx context.Context, id string) (*Order, *APIError) {
	return loadOrderTx(ctx, s.Pool, id)
}

func loadOrderTx(ctx context.Context, q dbQuerier, id string) (*Order, *APIError) {
	var o Order
	var orgID, addrID *string
	var created string
	err := q.QueryRow(ctx, `SELECT id::text, user_id::text, org_id::text, status, subtotal_cents,
		discount_cents, total_cents, COALESCE(coupon_code,''), COALESCE(payment_mode,'approve'),
		COALESCE(payment_auth_code,''), address_id::text, stock_reserved, created_at::text
		FROM orders WHERE id::text = $1`, id).Scan(
		&o.ID, &o.UserID, &orgID, &o.Status, &o.SubtotalCents, &o.DiscountCents, &o.TotalCents,
		&o.CouponCode, &o.PaymentMode, &o.PaymentAuthCode, &addrID, &o.StockReserved, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("order not found")
	}
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to load order"}
	}
	if orgID != nil {
		o.OrgID = *orgID
	}
	o.AddressID = addrID
	o.CreatedAt = created
	items, apiErr := loadItemsTx(ctx, q, o.ID)
	if apiErr != nil {
		return nil, apiErr
	}
	o.Items = items
	return &o, nil
}

func loadItemsTx(ctx context.Context, q dbQuerier, orderID string) ([]OrderItem, *APIError) {
	rows, err := q.Query(ctx, `SELECT id::text, order_id::text, COALESCE(product_id::text,''),
		COALESCE(product_name,''), qty, unit_price_cents
		FROM order_items WHERE order_id::text = $1 ORDER BY product_name`, orderID)
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	defer rows.Close()
	items := []OrderItem{}
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductName, &it.Qty, &it.UnitPriceCents); err != nil {
			return nil, &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	return items, nil
}

// ListMine returns the caller's orders, newest first, with items.
func (s *Store) ListMine(ctx context.Context, userID string, page, perPage int) ([]Order, int, *APIError) {
	if s.Pool == nil {
		return nil, 0, &APIError{Status: 503, Code: "unavailable", Message: "database not configured"}
	}
	page, perPage = clampPage(page, perPage)
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id::text = $1`, userID).Scan(&total); err != nil {
		return nil, 0, &APIError{Status: 500, Code: "internal", Message: "failed to count orders"}
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text, user_id::text, org_id::text, status, subtotal_cents,
		discount_cents, total_cents, COALESCE(coupon_code,''), COALESCE(payment_mode,'approve'),
		COALESCE(payment_auth_code,''), address_id::text, stock_reserved, created_at::text
		FROM orders WHERE user_id::text = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, &APIError{Status: 500, Code: "internal", Message: "failed to list orders"}
	}
	defer rows.Close()
	orders, apiErr := scanOrders(rows)
	if apiErr != nil {
		return nil, 0, apiErr
	}
	if err := attachItems(ctx, s.Pool, orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// Get returns one order if the caller owns it or is admin.
func (s *Store) Get(ctx context.Context, id, userID string, isAdmin bool) (*Order, *APIError) {
	if s.Pool == nil {
		return nil, &APIError{Status: 503, Code: "unavailable", Message: "database not configured"}
	}
	o, apiErr := s.loadOrder(ctx, id)
	if apiErr != nil {
		return nil, apiErr
	}
	if !isAdmin && o.UserID != userID {
		return nil, notFound("order not found")
	}
	return o, nil
}

// Cancel cancels an order. Owners may cancel pending/paid orders; admins may
// cancel pending/paid/shipped ones. Stock is restored (with movements) when the
// previous status held stock (pending/paid).
func (s *Store) Cancel(ctx context.Context, id, userID string, isAdmin bool) (*Order, *APIError) {
	if s.Pool == nil {
		return nil, &APIError{Status: 503, Code: "unavailable", Message: "database not configured"}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to begin transaction"}
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var o Order
	var orgID, addrID *string
	err = tx.QueryRow(ctx, `SELECT id::text, user_id::text, org_id::text, status, subtotal_cents,
		discount_cents, total_cents, COALESCE(coupon_code,''), COALESCE(payment_mode,'approve'),
		COALESCE(payment_auth_code,''), address_id::text, stock_reserved, created_at::text
		FROM orders WHERE id::text = $1 FOR UPDATE`, id).Scan(
		&o.ID, &o.UserID, &orgID, &o.Status, &o.SubtotalCents, &o.DiscountCents, &o.TotalCents,
		&o.CouponCode, &o.PaymentMode, &o.PaymentAuthCode, &addrID, &o.StockReserved, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("order not found")
	}
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to load order"}
	}
	if !isAdmin && o.UserID != userID {
		return nil, notFound("order not found")
	}
	switch o.Status {
	case StatusCancelled:
		return nil, conflict("already_cancelled", "order is already cancelled")
	case StatusFailed:
		return nil, conflict("cannot_cancel", "failed orders cannot be cancelled")
	case StatusPending, StatusPaid:
		// cancellable by owner and admin.
	case StatusShipped:
		if !isAdmin {
			return nil, conflict("cannot_cancel", "only pending/paid orders can be cancelled")
		}
	default:
		return nil, &APIError{Status: 500, Code: "internal", Message: "unknown order status"}
	}

	heldStock := o.StockReserved && (o.Status == StatusPending || o.Status == StatusPaid)
	if heldStock {
		if apiErr := restoreStockTx(ctx, tx, o.ID, "cancel:"+o.ID); apiErr != nil {
			return nil, apiErr
		}
	} else if _, err := tx.Exec(ctx, `UPDATE orders SET stock_reserved = false WHERE id::text = $1`, o.ID); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to release stock reservation"}
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'cancelled' WHERE id::text = $1`, o.ID); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to cancel order"}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to commit cancel"}
	}
	return s.loadOrder(ctx, o.ID)
}

// restoreStockTx returns the items' quantities to stock (locking product rows)
// and records compensating movements with the given reason.
func restoreStockTx(ctx context.Context, tx pgx.Tx, orderID, reason string) *APIError {
	rows, err := tx.Query(ctx, `SELECT product_id::text, qty FROM order_items WHERE order_id::text = $1`, orderID)
	if err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	type line struct {
		pid string
		qty int
	}
	lines := []line{}
	for rows.Next() {
		var l line
		var pid *string
		if err := rows.Scan(&pid, &l.qty); err != nil {
			rows.Close()
			return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
		}
		if pid != nil {
			l.pid = *pid
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	ids := []string{}
	for _, l := range lines {
		if l.pid != "" {
			ids = append(ids, l.pid)
		}
	}
	if len(ids) > 0 {
		sort.Strings(ids)
		if _, err := tx.Exec(ctx, `SELECT id FROM products WHERE id::text = ANY($1) ORDER BY id FOR UPDATE`, ids); err != nil {
			return &APIError{Status: 500, Code: "internal", Message: "failed to lock products"}
		}
	}
	for _, l := range lines {
		if l.pid == "" {
			continue // product deleted since; nothing to restore.
		}
		if _, err := tx.Exec(ctx, `UPDATE products SET stock_qty = stock_qty + $2, updated_at = now()
			WHERE id::text = $1`, l.pid, l.qty); err != nil {
			return &APIError{Status: 500, Code: "internal", Message: "failed to restore stock"}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stock_movements(product_id, order_id, qty, reason)
			VALUES($1::uuid, $2::uuid, $3, $4)`, l.pid, orderID, l.qty, reason); err != nil {
			return &APIError{Status: 500, Code: "internal", Message: "failed to record stock movement"}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET stock_reserved = false WHERE id::text = $1`, orderID); err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to release stock reservation"}
	}
	return nil
}

// acquireStockTx decrements stock for the order's items (locking product rows),
// records 'reserve:' movements and marks the reservation. Used when an admin
// marks a reopened (failed -> pending) order as paid.
func acquireStockTx(ctx context.Context, tx pgx.Tx, orderID string) *APIError {
	rows, err := tx.Query(ctx, `SELECT product_id::text, qty FROM order_items WHERE order_id::text = $1`, orderID)
	if err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	type line struct {
		pid string
		qty int
	}
	lines := []line{}
	for rows.Next() {
		var l line
		var pid *string
		if err := rows.Scan(&pid, &l.qty); err != nil {
			rows.Close()
			return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
		}
		if pid == nil || *pid == "" {
			rows.Close()
			return conflict("cannot_fulfill", "a product from this order no longer exists")
		}
		l.pid = *pid
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	ids := make([]string, 0, len(lines))
	for _, l := range lines {
		ids = append(ids, l.pid)
	}
	sort.Strings(ids)
	prows, err := tx.Query(ctx, `SELECT id::text, name, stock_qty FROM products
		WHERE id::text = ANY($1) ORDER BY id FOR UPDATE`, ids)
	if err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to lock products"}
	}
	stock := map[string]int{}
	names := map[string]string{}
	for prows.Next() {
		var id, name string
		var qty int
		if err := prows.Scan(&id, &name, &qty); err != nil {
			prows.Close()
			return &APIError{Status: 500, Code: "internal", Message: "failed to read products"}
		}
		stock[id] = qty
		names[id] = name
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to read products"}
	}
	for _, l := range lines {
		have, ok := stock[l.pid]
		if !ok {
			return conflict("cannot_fulfill", "a product from this order no longer exists")
		}
		if have < l.qty {
			return conflict("insufficient_stock",
				fmt.Sprintf("insufficient stock for %s: have %d, want %d", names[l.pid], have, l.qty))
		}
		if _, err := tx.Exec(ctx, `UPDATE products SET stock_qty = stock_qty - $2, updated_at = now()
			WHERE id::text = $1`, l.pid, l.qty); err != nil {
			return &APIError{Status: 500, Code: "internal", Message: "failed to decrement stock"}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stock_movements(product_id, order_id, qty, reason)
			VALUES($1::uuid, $2::uuid, $3, $4)`, l.pid, orderID, -l.qty, "reserve:"+orderID); err != nil {
			return &APIError{Status: 500, Code: "internal", Message: "failed to record stock movement"}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET stock_reserved = true WHERE id::text = $1`, orderID); err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to mark stock reservation"}
	}
	return nil
}

// AdminList lists orders with an optional status filter, newest first.
func (s *Store) AdminList(ctx context.Context, status string, page, perPage int) ([]Order, int, *APIError) {
	if s.Pool == nil {
		return nil, 0, &APIError{Status: 503, Code: "unavailable", Message: "database not configured"}
	}
	if status != "" && !validStatus(status) {
		return nil, 0, badRequest("invalid_status", "unknown status filter")
	}
	page, perPage = clampPage(page, perPage)
	filter := ""
	args := []any{}
	if status != "" {
		filter = "WHERE status = $1"
		args = append(args, status)
	}
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders `+filter, args...).Scan(&total); err != nil {
		return nil, 0, &APIError{Status: 500, Code: "internal", Message: "failed to count orders"}
	}
	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.Pool.Query(ctx, `SELECT id::text, user_id::text, org_id::text, status, subtotal_cents,
		discount_cents, total_cents, COALESCE(coupon_code,''), COALESCE(payment_mode,'approve'),
		COALESCE(payment_auth_code,''), address_id::text, stock_reserved, created_at::text
		FROM orders `+filter+` ORDER BY created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)),
		args...)
	if err != nil {
		return nil, 0, &APIError{Status: 500, Code: "internal", Message: "failed to list orders"}
	}
	defer rows.Close()
	orders, apiErr := scanOrders(rows)
	if apiErr != nil {
		return nil, 0, apiErr
	}
	if err := attachItems(ctx, s.Pool, orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// AdminSetStatus transitions an order per the documented matrix:
//   - same status → no-op; cancelled/failed are terminal (409), except
//     failed → pending (reopen, no stock change);
//   - to paid only from pending; to shipped only from paid;
//   - to cancelled/failed from pending/paid restores stock, from shipped does not.
func (s *Store) AdminSetStatus(ctx context.Context, id, target string) (*Order, *APIError) {
	if s.Pool == nil {
		return nil, &APIError{Status: 503, Code: "unavailable", Message: "database not configured"}
	}
	if !validStatus(target) {
		return nil, badRequest("invalid_status", "status must be pending|paid|failed|cancelled|shipped")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to begin transaction"}
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current string
	var reserved bool
	err = tx.QueryRow(ctx, `SELECT status, stock_reserved FROM orders WHERE id::text = $1 FOR UPDATE`, id).Scan(&current, &reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("order not found")
	}
	if err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to load order"}
	}
	if current == target {
		if err := tx.Commit(ctx); err != nil {
			return nil, &APIError{Status: 500, Code: "internal", Message: "failed to commit"}
		}
		return s.loadOrder(ctx, id)
	}
	switch target {
	case StatusPending:
		if current != StatusFailed {
			return nil, conflict("invalid_transition", "only failed orders can be reopened to pending")
		}
	case StatusPaid:
		if current != StatusPending {
			return nil, conflict("invalid_transition", "only pending orders can be marked paid")
		}
		// Reopened orders hold no stock: acquire it now (409 when unavailable).
		if !reserved {
			if apiErr := acquireStockTx(ctx, tx, id); apiErr != nil {
				return nil, apiErr
			}
		}
	case StatusShipped:
		if current != StatusPaid {
			return nil, conflict("invalid_transition", "only paid orders can be shipped")
		}
	case StatusCancelled, StatusFailed:
		switch {
		case reserved && (current == StatusPending || current == StatusPaid):
			if apiErr := restoreStockTx(ctx, tx, id, "cancel:"+id); apiErr != nil {
				return nil, apiErr
			}
		case current == StatusPending || current == StatusPaid || current == StatusShipped:
			// Nothing held (reopened order) or goods already shipped:
			// flip status, release the marker without touching stock.
			if _, err := tx.Exec(ctx, `UPDATE orders SET stock_reserved = false WHERE id::text = $1`, id); err != nil {
				return nil, &APIError{Status: 500, Code: "internal", Message: "failed to update order status"}
			}
		default:
			return nil, conflict("invalid_transition", "order is in a terminal state")
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET status = $2 WHERE id::text = $1`, id, target); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to update order status"}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to commit status change"}
	}
	return s.loadOrder(ctx, id)
}

func scanOrders(rows pgx.Rows) ([]Order, *APIError) {
	orders := []Order{}
	for rows.Next() {
		var o Order
		var orgID, addrID *string
		if err := rows.Scan(&o.ID, &o.UserID, &orgID, &o.Status, &o.SubtotalCents, &o.DiscountCents,
			&o.TotalCents, &o.CouponCode, &o.PaymentMode, &o.PaymentAuthCode, &addrID, &o.StockReserved, &o.CreatedAt); err != nil {
			return nil, &APIError{Status: 500, Code: "internal", Message: "failed to list orders"}
		}
		if orgID != nil {
			o.OrgID = *orgID
		}
		o.AddressID = addrID
		o.Items = []OrderItem{}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, &APIError{Status: 500, Code: "internal", Message: "failed to list orders"}
	}
	return orders, nil
}

func attachItems(ctx context.Context, q dbQuerier, orders []Order) *APIError {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]string, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	rows, err := q.Query(ctx, `SELECT id::text, order_id::text, COALESCE(product_id::text,''),
		COALESCE(product_name,''), qty, unit_price_cents
		FROM order_items WHERE order_id::text = ANY($1) ORDER BY product_name`, ids)
	if err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	defer rows.Close()
	byOrder := map[string][]OrderItem{}
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductName, &it.Qty, &it.UnitPriceCents); err != nil {
			return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
		}
		byOrder[it.OrderID] = append(byOrder[it.OrderID], it)
	}
	if err := rows.Err(); err != nil {
		return &APIError{Status: 500, Code: "internal", Message: "failed to load order items"}
	}
	for i := range orders {
		if items, ok := byOrder[orders[i].ID]; ok {
			orders[i].Items = items
		}
	}
	return nil
}

func clampPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
