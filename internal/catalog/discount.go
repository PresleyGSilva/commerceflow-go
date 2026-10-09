package catalog

import "time"

// Coupon is the testable in-memory view of a coupons row.
type Coupon struct {
	Code             string
	Type             string // "percent" | "fixed"
	Value            int
	MinSubtotalCents int
	Active           bool
	ExpiresAt        *time.Time
}

// CalcDiscount returns the discount in cents for a subtotal in cents.
// Rules: inactive/expired/below-minimum => 0; percent (floor) => subtotal*value/100;
// fixed => min(value, subtotal). Never negative.
func CalcDiscount(subtotal int, c Coupon, now time.Time) int {
	if subtotal <= 0 {
		return 0
	}
	if !c.Active {
		return 0
	}
	if c.ExpiresAt != nil && now.After(*c.ExpiresAt) {
		return 0
	}
	if subtotal < c.MinSubtotalCents {
		return 0
	}
	switch c.Type {
	case "percent":
		if c.Value <= 0 {
			return 0
		}
		v := c.Value
		if v > 100 {
			v = 100
		}
		return (subtotal * v) / 100
	case "fixed":
		if c.Value <= 0 {
			return 0
		}
		if c.Value > subtotal {
			return subtotal
		}
		return c.Value
	default:
		return 0
	}
}

// ValidateCouponInput enforces API rules: percent value 1-90, fixed value > 0,
// min_subtotal_cents >= 0, type percent|fixed.
func ValidateCouponInput(typ string, value, minSubtotal int) string {
	switch typ {
	case "percent":
		if value < 1 || value > 90 {
			return "value must be between 1 and 90 for percent coupons"
		}
	case "fixed":
		if value <= 0 {
			return "value must be positive (cents) for fixed coupons"
		}
	default:
		return "type must be percent or fixed"
	}
	if minSubtotal < 0 {
		return "min_subtotal_cents must be >= 0"
	}
	return ""
}
