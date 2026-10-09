package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Smoke: building the full router must not panic (chi rejects two mounts on
// the same path — catalog admin routes live inside the single /admin group)
// and the catalog paths must resolve (nil DB => 503/401, never 404).
func TestRouterMountsWithoutPanic(t *testing.T) {
	var r http.Handler
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("NewRouter panicked: %v", rec)
			}
		}()
		r = NewRouter(Deps{JWTSecret: "test-secret"})
	}()

	for _, target := range []string{"/api/v1/products", "/api/v1/categories", "/api/v1/coupons/validate?code=X"} {
		req := httptest.NewRequest("GET", target, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Errorf("GET %s: got 404", target)
		}
	}
}
