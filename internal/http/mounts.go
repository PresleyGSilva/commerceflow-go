package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/PresleyGSilva/commerceflow-go/internal/admin"
	"github.com/PresleyGSilva/commerceflow-go/internal/auth"
	"github.com/PresleyGSilva/commerceflow-go/internal/catalog"
	"github.com/PresleyGSilva/commerceflow-go/internal/orders"
)

// Mount hooks let feature slices register routes without editing the router.
// Default implementations are no-ops so the base builds standalone.
func MountCatalog(r chi.Router, d Deps) {
	// Inject JWT identity lookup (auth uses private context keys).
	catalog.UserIDFromRequest = auth.UserID
	catalog.UserRoleFromRequest = auth.UserRole
	catalog.Mount(r, d.DB)
}
func MountOrders(r chi.Router, d Deps) { orders.Mount(r, d.DB, d.JWTSecret) }
func MountAdmin(r chi.Router, d Deps) {
	ah := admin.New(d.DB)
	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.Middleware(d.JWTSecret))
		r.Use(auth.RequireRole("admin"))
		r.Get("/dashboard", ah.Dashboard)
		r.Get("/reports/sales", ah.SalesReport)
		r.Get("/reports/stock", ah.StockReport)
		// Catalog admin routes (no extra /admin wrapper: single chi mount).
		catalog.MountAdminRoutes(r, d.DB)
	})
}
