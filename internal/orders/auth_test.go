package orders

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/PresleyGSilva/commerceflow-go/internal/auth"
)

const authzSecret = "test-secret-orders-authz"

// authzRouter builds the real route stack with a nil pool: requests rejected
// by auth/role/validation never touch the database, so these tests always run.
func authzRouter() *chi.Mux {
	r := chi.NewRouter()
	Mount(r, nil, authzSecret)
	return r
}

func token(t *testing.T, userID, role, orgID string) string {
	t.Helper()
	tok, err := auth.SignAccess(authzSecret, userID, userID+"@exemplo.com", role, orgID)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

func doReq(r http.Handler, method, path, tok, orgID, idemKey, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if orgID != "" {
		req.Header.Set("X-Org-ID", orgID)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func errCode(t *testing.T, rr *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, rr.Body.String())
	}
	em, _ := payload["error"].(map[string]any)
	code, _ := em["code"].(string)
	return rr.Code, code
}

func TestAuthZCheckout(t *testing.T) {
	r := authzRouter()
	customer := token(t, "cust-1", "customer", "")
	body := `{"items":[{"product_id":"x","qty":1}],"payment_mode":"approve"}`

	if code, _ := errCode(t, doReq(r, "POST", "/checkout", "", "", "", body)); code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", code)
	}
	if code, ec := errCode(t, doReq(r, "POST", "/checkout", customer, "", "", "")); code != http.StatusBadRequest || ec != "missing_org" {
		t.Fatalf("no org: status = %d code = %s, want 400 missing_org", code, ec)
	}
	if code, ec := errCode(t, doReq(r, "POST", "/checkout", customer, "org-1", "", body)); code != http.StatusBadRequest || ec != "missing_idempotency_key" {
		t.Fatalf("no key: status = %d code = %s, want 400 missing_idempotency_key", code, ec)
	}
	badMode := `{"items":[{"product_id":"x","qty":1}],"payment_mode":"pix"}`
	if code, ec := errCode(t, doReq(r, "POST", "/checkout", customer, "org-1", "k-1", badMode)); code != http.StatusBadRequest || ec != "invalid_payment_mode" {
		t.Fatalf("bad mode: status = %d code = %s, want 400 invalid_payment_mode", code, ec)
	}
}

func TestAuthZOrders(t *testing.T) {
	r := authzRouter()
	customer := token(t, "cust-1", "customer", "")

	if code, _ := errCode(t, doReq(r, "GET", "/orders", "", "org-1", "", "")); code != http.StatusUnauthorized {
		t.Fatalf("GET /orders no token: status = %d, want 401", code)
	}
	if code, _ := errCode(t, doReq(r, "GET", "/orders/abc", customer, "org-1", "", "")); code != http.StatusServiceUnavailable {
		// Authenticated + tenant OK: passes auth, fails only on nil pool.
		t.Fatalf("GET /orders/:id customer: status = %d, want 503 (nil pool)", code)
	}
}

func TestAuthZAdminOrders(t *testing.T) {
	r := authzRouter()
	customer := token(t, "cust-1", "customer", "")
	admin := token(t, "admin-1", "admin", "")

	if code, _ := errCode(t, doReq(r, "GET", "/admin/orders", "", "", "", "")); code != http.StatusUnauthorized {
		t.Fatalf("admin list no token: status = %d, want 401", code)
	}
	if code, ec := errCode(t, doReq(r, "GET", "/admin/orders", customer, "", "", "")); code != http.StatusForbidden || ec != "forbidden" {
		t.Fatalf("admin list customer: status = %d code = %s, want 403 forbidden", code, ec)
	}
	if code, ec := errCode(t, doReq(r, "PATCH", "/admin/orders/abc/status", customer, "", "", `{"status":"shipped"}`)); code != http.StatusForbidden || ec != "forbidden" {
		t.Fatalf("admin patch customer: status = %d code = %s, want 403 forbidden", code, ec)
	}
	// Admin role passes: nil pool yields 503, proving routing + role check passed.
	if code, _ := errCode(t, doReq(r, "GET", "/admin/orders", admin, "", "", "")); code != http.StatusServiceUnavailable {
		t.Fatalf("admin list admin: status = %d, want 503 (nil pool)", code)
	}
}
