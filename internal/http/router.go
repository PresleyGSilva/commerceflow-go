package http

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PresleyGSilva/commerceflow-go/internal/auth"
)

// Deps wires shared dependencies into the router.
type Deps struct {
	DB          *pgxpool.Pool
	JWTSecret   string
	RefreshTTLH int
}

// NewRouter builds the chi router. Slice owners mount additional
// sub-routers via MountCatalog/MountOrders/MountAdmin hooks (no-op by default).
func NewRouter(d Deps) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(Logger)
	r.Use(CORS)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		OK(w, map[string]any{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		if d.DB != nil {
			if err := d.DB.Ping(ctx); err != nil {
				Error(w, http.StatusServiceUnavailable, "not_ready", "database unreachable")
				return
			}
		}
		OK(w, map[string]any{"status": "ready"})
	})
	r.Get("/openapi.yaml", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		http.ServeFile(w, req, "openapi.yaml")
	})

	ah := &auth.Handler{DB: d.DB, JWTSecret: d.JWTSecret, RefreshTTLH: d.RefreshTTLH}
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", ah.Register)
			r.Post("/login", ah.Login)
			r.Post("/refresh", ah.Refresh)
			r.Post("/logout", ah.Logout)
			r.With(auth.Middleware(d.JWTSecret)).Get("/me", ah.Me)
		})
		// Slice mount points: catalog, orders, admin register their routes here.
		MountCatalog(r, d)
		MountOrders(r, d)
		MountAdmin(r, d)
	})
	return r
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http request",
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allow := os.Getenv("CORS_ORIGIN")
		// Browsers reject credentialed requests (fetch credentials:"include",
		// needed for the httpOnly refresh cookie) when the server answers
		// Access-Control-Allow-Origin:*. Reflect the request origin instead.
		if allow == "" || allow == "*" {
			if origin != "" {
				allow = origin
			} else if allow == "" {
				allow = "*"
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", allow)
		if origin != "" {
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key,X-Org-ID,X-User")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
