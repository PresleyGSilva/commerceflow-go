package catalog

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Mount registers PUBLIC catalog routes on r, RELATIVE to the caller's group.
// The base router calls it inside r.Route("/api/v1", ...) (see
// internal/http/router.go + mounts.go), so Mount MUST NOT add its own
// "/api/v1" wrapper — that produced /api/v1/api/v1/* and 404s on E2E.
func Mount(r chi.Router, pool *pgxpool.Pool) {
	s := New(pool)

	r.Get("/categories", s.listCategories)
	r.Get("/products", s.listProducts)
	r.Get("/products/{slug}", s.getProduct)
	r.Get("/coupons/validate", s.validateCoupon)

	r.Route("/addresses", func(r chi.Router) {
		r.Get("/", s.listAddresses)
		r.Post("/", s.createAddress)
		r.Put("/{id}", s.updateAddress)
		r.Delete("/{id}", s.deleteAddress)
	})
}

// MountAdminRoutes registers catalog ADMIN routes on r, RELATIVE to the
// caller's /admin group — WITHOUT its own "/admin" wrapper. chi panics
// when two sub-routers mount on the same path, so the base mounts.go calls
// this INSIDE its single existing r.Route("/admin", ...) group (which also
// enforces Bearer auth + RequireRole("admin") at router level).
func MountAdminRoutes(r chi.Router, pool *pgxpool.Pool) {
	s := New(pool)

	r.Post("/categories", s.createCategory)
	r.Put("/categories/{id}", s.updateCategory)
	r.Delete("/categories/{id}", s.deleteCategory)

	r.Post("/products", s.createProduct)
	r.Put("/products/{id}", s.updateProduct)
	r.Delete("/products/{id}", s.deleteProduct)
	r.Patch("/products/{id}/stock", s.adjustStock)

	r.Get("/stock/movements", s.listMovements)

	r.Get("/coupons", s.listCoupons)
	r.Post("/coupons", s.createCoupon)
	r.Put("/coupons/{id}", s.updateCoupon)
	r.Delete("/coupons/{id}", s.deleteCoupon)
}
