package catalog

import (
	"testing"
	"time"
)

func TestCalcDiscountPercent(t *testing.T) {
	now := time.Now()
	got := CalcDiscount(10000, Coupon{Code: "BEMVINDO10", Type: "percent", Value: 10, Active: true}, now)
	if got != 1000 {
		t.Fatalf("expected 1000, got %d", got)
	}
}

func TestCalcDiscountPercentFloor(t *testing.T) {
	now := time.Now()
	// 199 * 10 / 100 = 19.9 -> floor 19
	got := CalcDiscount(199, Coupon{Type: "percent", Value: 10, Active: true}, now)
	if got != 19 {
		t.Fatalf("expected 19, got %d", got)
	}
}

func TestCalcDiscountFixed(t *testing.T) {
	now := time.Now()
	got := CalcDiscount(10000, Coupon{Type: "fixed", Value: 500, Active: true}, now)
	if got != 500 {
		t.Fatalf("expected 500, got %d", got)
	}
}

func TestCalcDiscountFixedCappedAtSubtotal(t *testing.T) {
	now := time.Now()
	got := CalcDiscount(300, Coupon{Type: "fixed", Value: 500, Active: true}, now)
	if got != 300 {
		t.Fatalf("expected 300 (capped), got %d", got)
	}
}

func TestCalcDiscountMinSubtotal(t *testing.T) {
	now := time.Now()
	got := CalcDiscount(400, Coupon{Type: "percent", Value: 10, MinSubtotalCents: 500, Active: true}, now)
	if got != 0 {
		t.Fatalf("expected 0 below min_subtotal, got %d", got)
	}
	got = CalcDiscount(500, Coupon{Type: "percent", Value: 10, MinSubtotalCents: 500, Active: true}, now)
	if got != 50 {
		t.Fatalf("expected 50 at min_subtotal boundary, got %d", got)
	}
}

func TestCalcDiscountInactive(t *testing.T) {
	now := time.Now()
	got := CalcDiscount(10000, Coupon{Type: "percent", Value: 10, Active: false}, now)
	if got != 0 {
		t.Fatalf("expected 0 for inactive, got %d", got)
	}
}

func TestCalcDiscountExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	got := CalcDiscount(10000, Coupon{Type: "percent", Value: 10, Active: true, ExpiresAt: &past}, now)
	if got != 0 {
		t.Fatalf("expected 0 for expired, got %d", got)
	}
	future := now.Add(time.Hour)
	got = CalcDiscount(10000, Coupon{Type: "percent", Value: 10, Active: true, ExpiresAt: &future}, now)
	if got != 1000 {
		t.Fatalf("expected 1000 for valid coupon, got %d", got)
	}
}

func TestCalcDiscountUnknownType(t *testing.T) {
	now := time.Now()
	if got := CalcDiscount(10000, Coupon{Type: "bogus", Value: 10, Active: true}, now); got != 0 {
		t.Fatalf("expected 0 for unknown type, got %d", got)
	}
	if got := CalcDiscount(0, Coupon{Type: "fixed", Value: 500, Active: true}, now); got != 0 {
		t.Fatalf("expected 0 for zero subtotal, got %d", got)
	}
}

func TestValidateCouponInput(t *testing.T) {
	if msg := ValidateCouponInput("percent", 10, 0); msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	if msg := ValidateCouponInput("percent", 0, 0); msg == "" {
		t.Fatal("expected error for percent 0")
	}
	if msg := ValidateCouponInput("percent", 91, 0); msg == "" {
		t.Fatal("expected error for percent 91")
	}
	if msg := ValidateCouponInput("fixed", 500, 0); msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	if msg := ValidateCouponInput("fixed", 0, 0); msg == "" {
		t.Fatal("expected error for fixed 0")
	}
	if msg := ValidateCouponInput("fixed", 500, -1); msg == "" {
		t.Fatal("expected error for negative min_subtotal")
	}
	if msg := ValidateCouponInput("other", 10, 0); msg == "" {
		t.Fatal("expected error for unknown type")
	}
}
