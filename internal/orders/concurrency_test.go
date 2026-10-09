package orders

import (
	"context"
	"sync"
	"testing"
)

// Two buyers racing for the last unit: exactly one checkout succeeds, stock
// never goes negative.
func TestConcurrentLastUnit(t *testing.T) {
	pool := testPool(t)
	ensureOrdersSchema(t, pool)
	s := New(pool)
	ctx := context.Background()

	buyerA := seedUser(t, pool, "customer")
	buyerB := seedUser(t, pool, "customer")
	pid, _ := seedProduct(t, pool, 1500, 1)

	type result struct {
		order  *Order
		apiErr *APIError
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	for i, tc := range []struct {
		user string
		key  string
	}{{buyerA, uniq("race-a")}, {buyerB, uniq("race-b")}} {
		wg.Add(1)
		go func(i int, user, key string) {
			defer wg.Done()
			o, _, apiErr := s.Checkout(ctx, user, "",
				[]ItemInput{{ProductID: pid, Qty: 1}}, "", "", "approve", key)
			results[i] = result{order: o, apiErr: apiErr}
		}(i, tc.user, tc.key)
	}
	wg.Wait()

	paid := 0
	for _, r := range results {
		if r.apiErr == nil {
			paid++
			if r.order.Status != StatusPaid {
				t.Fatalf("successful checkout status = %s, want paid", r.order.Status)
			}
		} else if r.apiErr.Code != "insufficient_stock" {
			t.Fatalf("loser error = %v, want insufficient_stock", r.apiErr)
		}
	}
	if paid != 1 {
		t.Fatalf("paid checkouts = %d, want exactly 1", paid)
	}
	if got := stockOf(t, pool, pid); got != 0 {
		t.Fatalf("stock = %d, want 0", got)
	}
	if got := stockOf(t, pool, pid); got < 0 {
		t.Fatal("stock went negative")
	}
	var paidCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders o JOIN order_items i ON i.order_id = o.id
		WHERE i.product_id::text = $1 AND o.status = 'paid'`, pid).Scan(&paidCount); err != nil {
		t.Fatalf("paid count: %v", err)
	}
	if paidCount != 1 {
		t.Fatalf("paid orders for product = %d, want 1", paidCount)
	}
}
