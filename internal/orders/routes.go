package orders

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PresleyGSilva/commerceflow-go/internal/auth"
)

// Mount registers checkout, orders and admin/orders routes. Auth is enforced
// here (Bearer access token); admin routes additionally require the admin role.
// Called from the base router's MountOrders hook.
func Mount(r chi.Router, pool *pgxpool.Pool, secret string) {
	s := New(pool)

	r.With(auth.Middleware(secret)).Post("/checkout", s.handleCheckout)
	r.With(auth.Middleware(secret)).Post("/checkout/preview", s.handlePreview)

	r.With(auth.Middleware(secret)).Get("/orders", s.handleListMine)
	r.With(auth.Middleware(secret)).Get("/orders/{id}", s.handleGet)
	r.With(auth.Middleware(secret)).Post("/orders/{id}/cancel", s.handleCancel)

	r.With(auth.Middleware(secret), auth.RequireRole("admin")).Get("/admin/orders", s.handleAdminList)
	r.With(auth.Middleware(secret), auth.RequireRole("admin")).Patch("/admin/orders/{id}/status", s.handleAdminSetStatus)
}
