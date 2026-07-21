// Package web wires the HTTP server, routes, and dependencies.
package web

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/catalog"
	"github.com/titipdong/titipdong/internal/config"
	"github.com/titipdong/titipdong/internal/currency"
	"github.com/titipdong/titipdong/internal/customers"
	"github.com/titipdong/titipdong/internal/kyc"
	"github.com/titipdong/titipdong/internal/orders"
	"github.com/titipdong/titipdong/internal/requests"
	"github.com/titipdong/titipdong/internal/scan"
	"github.com/titipdong/titipdong/internal/trips"
)

// Server bundles all services and the HTTP router.
type Server struct {
	cfg       config.Config
	pool      *pgxpool.Pool
	auth      *auth.Service
	sessions  *scs.SessionManager
	customers *customers.Store
	trips     *trips.Store
	orders    *orders.Store
	catalog   *catalog.Store
	kyc       *kyc.Store
	requests  *requests.Store
	currency  *currency.Service
	scan      *scan.Service
}

// New constructs the Server with all stores wired up.
func New(cfg config.Config, pool *pgxpool.Pool) *Server {
	sessions := scs.New()
	// Use the in-memory store instead of the default cookie store so the
	// session can hold larger values (e.g. scan.Result for the order pre-fill)
	// without hitting the ~4KB cookie size limit. Sessions are still keyed by
	// a signed cookie; only the data lives server-side.
	sessions.Store = memstore.New()
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	sessions.Cookie.Path = "/"

	authSvc := auth.New(pool, sessions)
	return &Server{
		cfg:       cfg,
		pool:      pool,
		auth:      authSvc,
		sessions:  sessions,
		customers: customers.NewStore(pool),
		trips:     trips.NewStore(pool),
		orders:    orders.NewStore(pool),
		catalog:   catalog.NewStore(pool),
		kyc:       kyc.NewStore(pool),
		requests:  requests.NewStore(pool),
		currency:  currency.New(pool, cfg.FXBaseURL),
		scan:      scan.New(cfg.OpenAIAPIKey),
	}
}

// logRecoverer wraps chi's Recoverer but also dumps the panic + stack trace
// to the server log, so "Internal Server Error" responses are debuggable
// (the default Recoverer swallows the panic silently).
func logRecoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rcv := recover(); rcv != nil {
				log.Printf("PANIC recovered: %s %s -> %v\n%s",
					r.Method, r.URL.Path, rcv, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

// Handler returns the configured HTTP handler.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.RequestID)
	r.Use(logRecoverer)
	r.Use(s.sessions.LoadAndSave)

	// Static assets.
	r.Get("/static/*", s.handleStatic())

	// PWA bits.
	r.Get("/manifest.json", s.handleManifest())
	r.Get("/sw.js", s.handleServiceWorker())

		// Public routes.
		r.Get("/", s.handleHome)
		r.Get("/login", s.handleLoginGet)
		r.Post("/login", s.handleLoginPost)
		r.Get("/signup", s.handleSignupGet)
		r.Post("/signup", s.handleSignupPost)
		r.Post("/logout", s.handleLogout)
		r.Get("/catalog", s.handleCatalogPublic)
		r.Get("/catalog/{id}/request", s.handleRequestForm)
		r.Post("/catalog/{id}/request", s.handleRequestSubmit)
		r.Get("/request", s.handleCustomRequestForm)
		r.Post("/request", s.handleCustomRequestSubmit)
		r.Get("/catalog/thanks", s.handleRequestThanks)
		r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

	// Authenticated area.
	r.Group(func(r chi.Router) {
		r.Use(s.auth.LoadUser)
		r.Use(s.auth.RequireAuth)

		// Buyer + above: profile, catalog, become-jastiper.
		r.Get("/app", s.handleAppHome)
		r.Get("/app/profile", s.handleProfile)
		r.Post("/app/profile/become-jastiper", s.handleBecomeJastiper)
		r.Get("/app/catalog", s.handleCatalog)
		r.Post("/app/catalog", s.handleCatalogCreate)
		r.Post("/app/catalog/{id}/delete", s.handleCatalogDelete)

		// Jastiper + admin: order management.
		r.Group(func(r chi.Router) {
			r.Use(s.auth.RequireRole(auth.RoleJastiper))

			// Customers.
			r.Get("/app/customers", s.handleCustomersList)
			r.Post("/app/customers", s.handleCustomerCreate)
			r.Get("/app/customers/{id}", s.handleCustomerEdit)
			r.Post("/app/customers/{id}", s.handleCustomerUpdate)
			r.Post("/app/customers/{id}/delete", s.handleCustomerDelete)

			// Trips.
			r.Get("/app/trips", s.handleTripsList)
			r.Post("/app/trips", s.handleTripCreate)
			r.Get("/app/trips/{id}", s.handleTripDashboard)
			r.Post("/app/trips/{id}/close", s.handleTripClose)

			// Orders.
			r.Get("/app/orders", s.handleOrdersList)
			r.Post("/app/orders", s.handleOrderCreate)
			r.Get("/app/orders/new", s.handleOrderNew)
			r.Get("/app/orders/fx", s.handleFXRate)
			r.Get("/app/orders/{id}", s.handleOrderEdit)
			r.Post("/app/orders/{id}", s.handleOrderUpdate)
			r.Post("/app/orders/{id}/delete", s.handleOrderDelete)
			r.Post("/app/orders/{id}/status", s.handleOrderStatusChange)
			r.Get("/app/orders/{id}/wa", s.handleOrderWhatsApp)
			r.Get("/app/orders/{id}/message", s.handleOrderMessage)

			// Receipt scan.
			r.Get("/app/scan", s.handleScanForm)
			r.Post("/app/scan", s.handleScan)

			// Payments.
			r.Get("/app/payments", s.handlePayments)

			// Summary.
			r.Get("/app/summary", s.handleSummary)

			// Buyer requests (anonymous catalog requests awaiting review).
			r.Get("/app/requests", s.handleRequestsList)
			r.Post("/app/requests/{id}/accept", s.handleRequestAccept)
			r.Post("/app/requests/{id}/reject", s.handleRequestReject)
			r.Get("/app/requests/{id}/wa", s.handleRequestWA)

			// Uploads (item/receipt/KTP photos).
			r.Get("/uploads/{name}", s.handleUpload)
		})

		// Admin only.
		r.Group(func(r chi.Router) {
			r.Use(s.auth.RequireRole())
			r.Get("/app/admin/users", s.handleAdminUsers)
			r.Post("/app/admin/users/{id}/role", s.handleAdminSetRole)
			r.Get("/app/admin/applications", s.handleAdminApplications)
			r.Post("/app/admin/rates/refresh", s.handleAdminRefreshRates)
			r.Get("/app/admin/rates", s.handleAdminRates)
			r.Get("/app/admin/orders", s.handleAdminOrders)
			r.Post("/app/admin/applications/{id}/approve", s.handleAdminApprove)
			r.Post("/app/admin/applications/{id}/reject", s.handleAdminReject)
		})
	})

	return r
}

// placeholder until handlers are wired.
func (s *Server) Shutdown(ctx context.Context) error {
	s.pool.Close()
	return nil
}
