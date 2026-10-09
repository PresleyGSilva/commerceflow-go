package orders

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to DATABASE_URL (or the compose default) and skips the
// test when no database is reachable.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://commerce:commerce@localhost:5435/commerceflow?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skipf("no database (dial): %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("no database (ping %s): %v", url, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// ensureOrdersSchema creates the (unprefixed) tables this slice needs when the
// full migration chain was not applied. It mirrors 001_core + 004_orders and
// is a no-op on databases where migrations already ran.
func ensureOrdersSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ddl := `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'customer' CHECK (role IN ('admin','customer')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
  stock_qty INTEGER NOT NULL DEFAULT 0 CHECK (stock_qty >= 0),
  category_id UUID,
  image_url TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS coupons (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code TEXT NOT NULL UNIQUE,
  type TEXT NOT NULL CHECK (type IN ('percent','fixed')),
  value INTEGER NOT NULL CHECK (value >= 0),
  min_subtotal_cents INTEGER NOT NULL DEFAULT 0 CHECK (min_subtotal_cents >= 0),
  active BOOLEAN NOT NULL DEFAULT true,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS addresses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label TEXT NOT NULL DEFAULT '',
  street TEXT NOT NULL DEFAULT '',
  city TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  org_id UUID,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','failed','cancelled','shipped')),
  subtotal_cents INTEGER NOT NULL DEFAULT 0 CHECK (subtotal_cents >= 0),
  discount_cents INTEGER NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
  total_cents INTEGER NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
  coupon_code TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  payment_auth_code TEXT NOT NULL DEFAULT '',
  address_id UUID REFERENCES addresses(id) ON DELETE SET NULL,
  stock_reserved BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_user_idem ON orders(user_id, idempotency_key) WHERE idempotency_key <> '';
CREATE TABLE IF NOT EXISTS order_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  product_id UUID REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL DEFAULT '',
  qty INTEGER NOT NULL CHECK (qty > 0),
  unit_price_cents INTEGER NOT NULL CHECK (unit_price_cents >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS stock_movements (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
  qty INTEGER NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`
	ctx := context.Background()
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
}

func uniq(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func seedUser(t *testing.T, pool *pgxpool.Pool, role string) string {
	t.Helper()
	id := uuid.NewString()
	email := uniq("u") + "@exemplo.com"
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users(id, email, password_hash, name, role) VALUES($1::uuid, $2, 'x', $3, $4)`,
		id, email, "Test User", role)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedProduct(t *testing.T, pool *pgxpool.Pool, priceCents, stock int) (id, slug string) {
	t.Helper()
	id = uuid.NewString()
	slug = uniq("p")
	_, err := pool.Exec(context.Background(),
		`INSERT INTO products(id, name, slug, price_cents, stock_qty) VALUES($1::uuid, $2, $3, $4, $5)`,
		id, "Produto "+slug, slug, priceCents, stock)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	return id, slug
}

func seedCoupon(t *testing.T, pool *pgxpool.Pool, typ string, value int) string {
	t.Helper()
	code := strings.ToUpper(uniq("C"))
	_, err := pool.Exec(context.Background(),
		`INSERT INTO coupons(code, type, value) VALUES($1, $2, $3)`, code, typ, value)
	if err != nil {
		t.Fatalf("seed coupon: %v", err)
	}
	return code
}

func stockOf(t *testing.T, pool *pgxpool.Pool, productID string) int {
	t.Helper()
	var s int
	if err := pool.QueryRow(context.Background(),
		`SELECT stock_qty FROM products WHERE id::text = $1`, productID).Scan(&s); err != nil {
		t.Fatalf("stockOf: %v", err)
	}
	return s
}

func movementSum(t *testing.T, pool *pgxpool.Pool, productID string) int {
	t.Helper()
	var sum int
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(qty),0) FROM stock_movements WHERE product_id::text = $1`, productID).Scan(&sum); err != nil {
		t.Fatalf("movementSum: %v", err)
	}
	return sum
}

func TestCheckoutApprove(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 1000, 10)
	coupon := seedCoupon(t, pool, "percent", 10)

	o, created, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 2}}, "", coupon, "approve", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout: %v", apiErr)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if o.Status != StatusPaid {
		t.Fatalf("status = %s, want paid", o.Status)
	}
	if o.SubtotalCents != 2000 || o.DiscountCents != 200 || o.TotalCents != 1800 {
		t.Fatalf("totals = %d/%d/%d, want 2000/200/1800", o.SubtotalCents, o.DiscountCents, o.TotalCents)
	}
	if !strings.HasPrefix(o.PaymentAuthCode, "AUTH-") {
		t.Fatalf("auth code = %q, want AUTH- prefixed", o.PaymentAuthCode)
	}
	if len(o.Items) != 1 || o.Items[0].Qty != 2 || o.Items[0].UnitPriceCents != 1000 || o.Items[0].ProductName == "" {
		t.Fatalf("items snapshot wrong: %+v", o.Items)
	}
	if got := stockOf(t, pool, pid); got != 8 {
		t.Fatalf("stock = %d, want 8", got)
	}
	if got := movementSum(t, pool, pid); got != -2 {
		t.Fatalf("movement sum = %d, want -2", got)
	}
}

func TestCheckoutDeclineKeepsStock(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 500, 5)

	o, created, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 2}}, "", "", "decline", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout: %v", apiErr)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if o.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", o.Status)
	}
	if o.PaymentAuthCode != "" {
		t.Fatalf("auth code = %q, want empty on decline", o.PaymentAuthCode)
	}
	if got := stockOf(t, pool, pid); got != 5 {
		t.Fatalf("stock = %d, want 5 (rolled back)", got)
	}
	if got := movementSum(t, pool, pid); got != 0 {
		t.Fatalf("movement sum = %d, want 0 (-2+2 rollback)", got)
	}
}

func TestCheckoutFailKeepsStock(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 700, 3)

	o, _, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 1}}, "", "", "fail", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout: %v", apiErr)
	}
	if o.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", o.Status)
	}
	if got := stockOf(t, pool, pid); got != 3 {
		t.Fatalf("stock = %d, want 3 (rolled back)", got)
	}
}

func TestCheckoutIdempotent(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 1200, 6)
	key := uniq("key")

	first, created, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 2}}, "", "", "approve", key)
	if apiErr != nil || !created {
		t.Fatalf("first checkout: %v created=%v", apiErr, created)
	}
	second, created, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 2}}, "", "", "approve", key)
	if apiErr != nil {
		t.Fatalf("second checkout: %v", apiErr)
	}
	if created {
		t.Fatal("second checkout created = true, want false (replay)")
	}
	if second.ID != first.ID {
		t.Fatalf("replay id = %s, want %s", second.ID, first.ID)
	}
	if got := stockOf(t, pool, pid); got != 4 {
		t.Fatalf("stock = %d, want 4 (decremented once)", got)
	}
}

func TestCheckoutValidation(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 100, 1)

	cases := []struct {
		name  string
		items []ItemInput
		mode  string
		key   string
		code  string
	}{
		{"empty cart", nil, "approve", uniq("k"), "empty_cart"},
		{"zero qty", []ItemInput{{ProductID: pid, Qty: 0}}, "approve", uniq("k"), "invalid_qty"},
		{"missing key", []ItemInput{{ProductID: pid, Qty: 1}}, "approve", "", "missing_idempotency_key"},
		{"bad mode", []ItemInput{{ProductID: pid, Qty: 1}}, "pix", uniq("k"), "invalid_payment_mode"},
		{"unknown product", []ItemInput{{ProductID: uuid.NewString(), Qty: 1}}, "approve", uniq("k"), "not_found"},
		{"insufficient stock", []ItemInput{{ProductID: pid, Qty: 2}}, "approve", uniq("k"), "insufficient_stock"},
		{"unknown coupon", []ItemInput{{ProductID: pid, Qty: 1}}, "approve", uniq("k"), "invalid_coupon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			coupon := ""
			if tc.name == "unknown coupon" {
				coupon = "NOPE123"
			}
			_, _, apiErr := s.Checkout(ctx, customer, "", tc.items, "", coupon, tc.mode, tc.key)
			if apiErr == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.code == "not_found" {
				if apiErr.Status != 404 {
					t.Fatalf("status = %d, want 404", apiErr.Status)
				}
				return
			}
			if apiErr.Code != tc.code {
				t.Fatalf("code = %s, want %s (%v)", apiErr.Code, tc.code, apiErr)
			}
		})
	}
	if got := stockOf(t, pool, pid); got != 1 {
		t.Fatalf("stock = %d, want 1 (validation never touches stock)", got)
	}
}

func TestPreview(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	pid, _ := seedProduct(t, pool, 1000, 10)
	coupon := seedCoupon(t, pool, "fixed", 500)

	var before int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&before)

	totals, apiErr := s.Preview(ctx, []ItemInput{{ProductID: pid, Qty: 3}}, coupon)
	if apiErr != nil {
		t.Fatalf("preview: %v", apiErr)
	}
	if totals.SubtotalCents != 3000 || totals.DiscountCents != 500 || totals.TotalCents != 2500 {
		t.Fatalf("totals = %+v, want 3000/500/2500", totals)
	}

	var after int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&after)
	if after != before {
		t.Fatal("preview persisted an order")
	}
	if got := stockOf(t, pool, pid); got != 10 {
		t.Fatalf("stock = %d, want 10 (preview is read-only)", got)
	}

	if _, apiErr := s.Preview(ctx, []ItemInput{{ProductID: pid, Qty: 1}}, "NOPE123"); apiErr == nil || apiErr.Code != "invalid_coupon" {
		t.Fatalf("preview unknown coupon: %v", apiErr)
	}
}

func TestCancelPaidRestoresStock(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	other := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 900, 4)

	o, _, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 3}}, "", "", "approve", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout: %v", apiErr)
	}

	// Stranger cannot cancel (hidden as 404).
	if _, apiErr := s.Cancel(ctx, o.ID, other, false); apiErr == nil || apiErr.Status != 404 {
		t.Fatalf("cancel by stranger: %v", apiErr)
	}

	cancelled, apiErr := s.Cancel(ctx, o.ID, customer, false)
	if apiErr != nil {
		t.Fatalf("cancel: %v", apiErr)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("status = %s, want cancelled", cancelled.Status)
	}
	if got := stockOf(t, pool, pid); got != 4 {
		t.Fatalf("stock = %d, want 4 (restored)", got)
	}

	// Double cancel → 409.
	if _, apiErr := s.Cancel(ctx, o.ID, customer, false); apiErr == nil || apiErr.Code != "already_cancelled" {
		t.Fatalf("double cancel: %v", apiErr)
	}
}

func TestOwnerCannotCancelShipped(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 900, 4)

	o, _, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 1}}, "", "", "approve", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout: %v", apiErr)
	}
	if _, apiErr := s.AdminSetStatus(ctx, o.ID, StatusShipped); apiErr != nil {
		t.Fatalf("ship: %v", apiErr)
	}
	if _, apiErr := s.Cancel(ctx, o.ID, customer, false); apiErr == nil || apiErr.Code != "cannot_cancel" {
		t.Fatalf("owner cancel shipped: %v", apiErr)
	}
	// Admin CAN cancel shipped (no stock restore — goods left the warehouse).
	cancelled, apiErr := s.Cancel(ctx, o.ID, customer, true)
	if apiErr != nil {
		t.Fatalf("admin cancel shipped: %v", apiErr)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("status = %s, want cancelled", cancelled.Status)
	}
	if got := stockOf(t, pool, pid); got != 3 {
		t.Fatalf("stock = %d, want 3 (no restore for shipped)", got)
	}
}

func TestGetOwnerOrAdmin(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	other := seedUser(t, pool, "customer")
	admin := seedUser(t, pool, "admin")
	pid, _ := seedProduct(t, pool, 100, 5)

	o, _, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 1}}, "", "", "approve", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout: %v", apiErr)
	}
	if _, apiErr := s.Get(ctx, o.ID, customer, false); apiErr != nil {
		t.Fatalf("owner get: %v", apiErr)
	}
	if _, apiErr := s.Get(ctx, o.ID, admin, true); apiErr != nil {
		t.Fatalf("admin get: %v", apiErr)
	}
	if _, apiErr := s.Get(ctx, o.ID, other, false); apiErr == nil || apiErr.Status != 404 {
		t.Fatalf("stranger get: %v", apiErr)
	}

	mine, total, apiErr := s.ListMine(ctx, customer, 1, 20)
	if apiErr != nil || total < 1 || len(mine) == 0 {
		t.Fatalf("list mine: %v total=%d", apiErr, total)
	}
	theirs, _, apiErr := s.ListMine(ctx, other, 1, 20)
	if apiErr != nil {
		t.Fatalf("list theirs: %v", apiErr)
	}
	for _, x := range theirs {
		if x.ID == o.ID {
			t.Fatal("order leaked into another user's list")
		}
	}
}

func TestAdminListAndStatus(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 200, 10)

	o1, _, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 1}}, "", "", "approve", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout1: %v", apiErr)
	}
	o2, _, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 1}}, "", "", "decline", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout2: %v", apiErr)
	}

	paid, total, apiErr := s.AdminList(ctx, StatusPaid, 1, 20)
	if apiErr != nil {
		t.Fatalf("admin list: %v", apiErr)
	}
	if total < 1 {
		t.Fatal("admin list paid: empty")
	}
	for _, x := range paid {
		if x.Status != StatusPaid {
			t.Fatalf("filter leak: status %s", x.Status)
		}
	}

	shipped, apiErr := s.AdminSetStatus(ctx, o1.ID, StatusShipped)
	if apiErr != nil {
		t.Fatalf("ship: %v", apiErr)
	}
	if shipped.Status != StatusShipped {
		t.Fatalf("status = %s, want shipped", shipped.Status)
	}

	// paid cannot go back to pending.
	if _, apiErr := s.AdminSetStatus(ctx, o1.ID, StatusPending); apiErr == nil || apiErr.Code != "invalid_transition" {
		t.Fatalf("paid->pending: %v", apiErr)
	}

	// failed can be reopened to pending (no stock movement: already rolled back).
	reopened, apiErr := s.AdminSetStatus(ctx, o2.ID, StatusPending)
	if apiErr != nil {
		t.Fatalf("reopen: %v", apiErr)
	}
	if reopened.Status != StatusPending {
		t.Fatalf("status = %s, want pending", reopened.Status)
	}

	// terminal states are immutable.
	if _, apiErr := s.AdminSetStatus(ctx, o2.ID, StatusFailed); apiErr != nil {
		t.Fatalf("pending->failed (admin cancel path): %v", apiErr)
	}
	if _, apiErr := s.AdminSetStatus(ctx, o2.ID, StatusPaid); apiErr == nil || apiErr.Code != "invalid_transition" {
		t.Fatalf("failed->paid: %v", apiErr)
	}

	// Cancelling the reopened-then-failed order must NOT restore stock twice:
	// stock was 10 - 1 (o1 paid) = 9; failed orders hold nothing.
	if got := stockOf(t, pool, pid); got != 9 {
		t.Fatalf("stock = %d, want 9", got)
	}
}

func TestCheckoutWithAddress(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	customer := seedUser(t, pool, "customer")
	other := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 300, 5)

	var addrID string
	if err := pool.QueryRow(ctx, `INSERT INTO addresses(user_id, label, street, city)
		VALUES($1::uuid, 'casa', 'Rua A', 'SP') RETURNING id::text`, customer).Scan(&addrID); err != nil {
		t.Fatalf("seed address: %v", err)
	}

	o, _, apiErr := s.Checkout(ctx, customer, "", []ItemInput{{ProductID: pid, Qty: 1}}, addrID, "", "approve", uniq("key"))
	if apiErr != nil {
		t.Fatalf("checkout with address: %v", apiErr)
	}
	if o.AddressID == nil || *o.AddressID != addrID {
		t.Fatalf("address_id = %v, want %s", o.AddressID, addrID)
	}

	// Someone else's address is rejected.
	if _, _, apiErr := s.Checkout(ctx, other, "", []ItemInput{{ProductID: pid, Qty: 1}}, addrID, "", "approve", uniq("key")); apiErr == nil || apiErr.Status != 404 {
		t.Fatalf("foreign address: %v", apiErr)
	}
}
