package catalog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// Regression 1: Mount must register RELATIVE routes. The base router calls
// MountCatalog inside r.Route("/api/v1", ...), so an extra "/api/v1"
// wrapper here produced /api/v1/api/v1/* and 404s on the real paths.
// Pool is nil: handlers answer 503/403, but MUST NOT 404.
func TestMountRegistersUnderApiV1Group(t *testing.T) {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		Mount(r, nil)
		r.Route("/admin", func(r chi.Router) { MountAdminRoutes(r, nil) })
	})

	public := []struct{ method, target string }{
		{"GET", "/api/v1/categories"},
		{"GET", "/api/v1/products"},
		{"GET", "/api/v1/products/some-slug"},
		{"GET", "/api/v1/coupons/validate?code=X&subtotal=100"},
		{"GET", "/api/v1/addresses"},
	}
	for _, c := range public {
		req := httptest.NewRequest(c.method, c.target, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s %s: got 404 (route not registered — double /api/v1 prefix?)", c.method, c.target)
		}
	}

	// Admin routes behind the single /admin mount (X-Role bypasses the
	// handler-level check; nil pool then yields 503 — anything but 404).
	admin := []struct{ method, target string }{
		{"POST", "/api/v1/admin/categories"},
		{"PUT", "/api/v1/admin/categories/1"},
		{"DELETE", "/api/v1/admin/categories/1"},
		{"POST", "/api/v1/admin/products"},
		{"PUT", "/api/v1/admin/products/1"},
		{"DELETE", "/api/v1/admin/products/1"},
		{"PATCH", "/api/v1/admin/products/1/stock"},
		{"GET", "/api/v1/admin/stock/movements"},
		{"GET", "/api/v1/admin/coupons"},
		{"POST", "/api/v1/admin/coupons"},
		{"PUT", "/api/v1/admin/coupons/1"},
		{"DELETE", "/api/v1/admin/coupons/1"},
	}
	for _, c := range admin {
		req := httptest.NewRequest(c.method, c.target, nil)
		req.Header.Set("X-Role", "admin")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s %s: got 404 (admin route not registered?)", c.method, c.target)
		}
	}

	// The old double-prefixed paths must NOT exist.
	for _, target := range []string{"/api/v1/api/v1/products", "/api/v1/api/v1/categories"} {
		req := httptest.NewRequest("GET", target, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: expected 404, got %d", target, rec.Code)
		}
	}
}
