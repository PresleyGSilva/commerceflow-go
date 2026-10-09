package orders

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/PresleyGSilva/commerceflow-go/internal/auth"
	"github.com/PresleyGSilva/commerceflow-go/internal/httpx"
	"github.com/PresleyGSilva/commerceflow-go/internal/payment"
)

func writeAPIError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		httpx.Error(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "internal", "unexpected error")
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return false
	}
	return true
}

// requireOrg enforces the X-Org-ID tenant header (auth.OrgID falls back to the
// JWT claim). X-User is optional and only logged.
func requireOrg(w http.ResponseWriter, r *http.Request) (string, bool) {
	orgID := auth.OrgID(r)
	if orgID == "" {
		httpx.Error(w, http.StatusBadRequest, "missing_org", "X-Org-ID header is required")
		return "", false
	}
	if xu := r.Header.Get("X-User"); xu != "" {
		slog.Info("tenant request", "org_id", orgID, "x_user", xu,
			"method", r.Method, "path", r.URL.Path)
	}
	return orgID, true
}

type cartBody struct {
	Items      []ItemInput `json:"items"`
	CouponCode string      `json:"coupon_code"`
}

// POST /api/v1/checkout/preview — totals without persisting.
func (s *Store) handlePreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOrg(w, r); !ok {
		return
	}
	var in cartBody
	if !decodeBody(w, r, &in) {
		return
	}
	totals, apiErr := s.Preview(r.Context(), in.Items, in.CouponCode)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	httpx.OK(w, totals)
}

type checkoutBody struct {
	Items       []ItemInput `json:"items"`
	AddressID   string      `json:"address_id"`
	CouponCode  string      `json:"coupon_code"`
	PaymentMode string      `json:"payment_mode"`
}

// POST /api/v1/checkout — Idempotency-Key header is REQUIRED.
func (s *Store) handleCheckout(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrg(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		httpx.Error(w, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required")
		return
	}
	var in checkoutBody
	if !decodeBody(w, r, &in) {
		return
	}
	if !payment.ValidMode(in.PaymentMode) {
		httpx.Error(w, http.StatusBadRequest, "invalid_payment_mode", "payment_mode must be approve|decline|fail")
		return
	}
	order, _, apiErr := s.Checkout(r.Context(), auth.UserID(r), orgID, in.Items,
		in.AddressID, in.CouponCode, in.PaymentMode, key)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	httpx.OK(w, order)
}

// GET /api/v1/orders — caller's own orders.
func (s *Store) handleListMine(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOrg(w, r); !ok {
		return
	}
	page, perPage := pageParams(r)
	orders, total, apiErr := s.ListMine(r.Context(), auth.UserID(r), page, perPage)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": orders,
		"meta": map[string]any{"page": page, "per_page": perPage, "total": total},
	})
}

// GET /api/v1/orders/{id} — owner or admin.
func (s *Store) handleGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOrg(w, r); !ok {
		return
	}
	isAdmin := auth.UserRole(r) == "admin"
	order, apiErr := s.Get(r.Context(), chi.URLParam(r, "id"), auth.UserID(r), isAdmin)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	httpx.OK(w, order)
}

// POST /api/v1/orders/{id}/cancel — owner (pending/paid) or admin.
func (s *Store) handleCancel(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOrg(w, r); !ok {
		return
	}
	isAdmin := auth.UserRole(r) == "admin"
	order, apiErr := s.Cancel(r.Context(), chi.URLParam(r, "id"), auth.UserID(r), isAdmin)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	httpx.OK(w, order)
}

// GET /api/v1/admin/orders?status=&page=&per_page= — admin only.
func (s *Store) handleAdminList(w http.ResponseWriter, r *http.Request) {
	page, perPage := pageParams(r)
	orders, total, apiErr := s.AdminList(r.Context(), r.URL.Query().Get("status"), page, perPage)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": orders,
		"meta": map[string]any{"page": page, "per_page": perPage, "total": total},
	})
}

// PATCH /api/v1/admin/orders/{id}/status — admin only.
func (s *Store) handleAdminSetStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	order, apiErr := s.AdminSetStatus(r.Context(), chi.URLParam(r, "id"), in.Status)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	httpx.OK(w, order)
}

func pageParams(r *http.Request) (page, perPage int) {
	page, perPage = 1, 20
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("per_page")); err == nil && v > 0 {
		perPage = v
	}
	return page, perPage
}
