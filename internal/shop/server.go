package shop

import (
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

// Config configures the HTTP handler.
type Config struct {
	Secret    string
	TokenTTL  time.Duration
	AccessLog bool
	UI        fs.FS // root of the built SPA (may be nil)
}

// Server is the loadshop HTTP application.
type Server struct {
	Catalog *Catalog
	Store   *Store
	Auth    *Auth
	Metrics *Metrics

	mux       *http.ServeMux
	accessLog bool
}

// New builds the application.
func New(cfg Config) (*Server, error) {
	cat, err := NewCatalog()
	if err != nil {
		return nil, err
	}
	site, err := newStaticSite(cfg.UI)
	if err != nil {
		return nil, err
	}
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = 24 * time.Hour
	}
	s := &Server{
		Catalog:   cat,
		Store:     NewStore(cat),
		Auth:      NewAuth(cfg.Secret, cfg.TokenTTL),
		Metrics:   newMetrics(),
		mux:       http.NewServeMux(),
		accessLog: cfg.AccessLog,
	}

	s.Metrics.register("OPTIONS /api/*") // CORS preflight, answered in ServeHTTP
	s.handle("GET /healthz", s.healthz)
	s.handle("GET /metrics", s.metrics)

	s.handle("POST /api/auth/login", s.login)
	s.handle("GET /api/auth/me", s.authed(s.me))
	s.handle("POST /api/auth/tokens", s.mintTokens)

	s.handle("GET /api/products", s.listProducts)
	s.handle("GET /api/products/{id}", s.getProduct)
	s.handle("GET /api/products/{id}/image.svg", s.productImage)
	s.handle("GET /api/categories", s.categories)

	s.handle("GET /api/cart", s.authed(s.getCart))
	s.handle("POST /api/cart/items", s.authed(s.addCartItem))
	s.handle("PATCH /api/cart/items/{productId}", s.authed(s.updateCartItem))
	s.handle("DELETE /api/cart/items/{productId}", s.authed(s.removeCartItem))

	s.handle("POST /api/orders", s.authed(s.checkout))
	s.handle("GET /api/orders", s.authed(s.listOrders))
	s.handle("GET /api/orders/{id}", s.authed(s.getOrder))

	s.handle("POST /api/admin/reset", s.reset)

	s.handle("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	s.handle("/", site.ServeHTTP)
	return s, nil
}

func (s *Server) handle(pattern string, h http.HandlerFunc) {
	s.Metrics.register(pattern)
	s.mux.HandleFunc(pattern, h)
}

// statusWriter records the response status for metrics.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// ServeHTTP applies CORS, metrics and optional access logging around the mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m := s.Metrics
	m.total.Add(1)
	m.inFlight.Add(1)
	defer m.inFlight.Add(-1)

	var start time.Time
	if s.accessLog {
		start = time.Now()
	}
	sw := &statusWriter{ResponseWriter: w}

	if strings.HasPrefix(r.URL.Path, "/api/") {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		h.Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			sw.WriteHeader(http.StatusNoContent)
			m.record("OPTIONS /api/*", sw.status)
			return
		}
	}

	s.mux.ServeHTTP(sw, r)
	if sw.status == 0 {
		sw.status = http.StatusOK
	}
	// ServeMux sets r.Pattern on the request it routed (Go 1.23+).
	m.record(r.Pattern, sw.status)
	if s.accessLog {
		log.Printf("%s %s %d %s %s", r.Method, r.URL.RequestURI(), sw.status, time.Since(start).Round(time.Microsecond), r.RemoteAddr)
	}
}
