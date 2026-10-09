package orders

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store backs the checkout/orders HTTP slice. Pool may be nil only in
// router-construction tests that never reach a handler.
type Store struct {
	Pool *pgxpool.Pool
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

// APIError is a service-level failure with an explicit HTTP mapping.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func badRequest(code, msg string) *APIError {
	return &APIError{Status: 400, Code: code, Message: msg}
}

func notFound(msg string) *APIError {
	return &APIError{Status: 404, Code: "not_found", Message: msg}
}

func conflict(code, msg string) *APIError {
	return &APIError{Status: 409, Code: code, Message: msg}
}

// ItemInput is one cart line: product + quantity.
type ItemInput struct {
	ProductID string `json:"product_id"`
	Qty       int    `json:"qty"`
}

// OrderItem is a persisted line with a price/name snapshot.
type OrderItem struct {
	ID             string `json:"id"`
	OrderID        string `json:"order_id"`
	ProductID      string `json:"product_id"`
	ProductName    string `json:"product_name"`
	Qty            int    `json:"qty"`
	UnitPriceCents int    `json:"unit_price_cents"`
}

// Order is the API view of an orders row with its items.
type Order struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	OrgID           string `json:"org_id"`
	Status          string `json:"status"`
	SubtotalCents   int    `json:"subtotal_cents"`
	DiscountCents   int    `json:"discount_cents"`
	TotalCents      int    `json:"total_cents"`
	CouponCode      string `json:"coupon_code"`
	PaymentMode     string `json:"payment_mode"`
	PaymentAuthCode string `json:"payment_auth_code,omitempty"`
	// StockReserved is true while the order holds decremented stock.
	StockReserved bool        `json:"stock_reserved"`
	AddressID     *string     `json:"address_id"`
	CreatedAt     string      `json:"created_at"`
	Items         []OrderItem `json:"items"`
}

// Order statuses.
const (
	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusShipped   = "shipped"
)

// validStatus reports whether s is a known order status.
func validStatus(s string) bool {
	switch s {
	case StatusPending, StatusPaid, StatusFailed, StatusCancelled, StatusShipped:
		return true
	default:
		return false
	}
}
