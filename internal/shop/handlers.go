package shop

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ctxKey struct{}

func userFrom(r *http.Request) string { return r.Context().Value(ctxKey{}).(string) }

// authed wraps a handler with bearer-token verification.
func (s *Server) authed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := bearer(r.Header.Get("Authorization"))
		if tok == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		user, err := s.Auth.Verify(tok)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	}
}

var healthzBody = []byte(`{"status":"ok"}`)

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeRaw(w, http.StatusOK, healthzBody)
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	s.Metrics.write(w)
}

// ---- auth ----

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Username    string `json:"username"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if len(req.Username) > maxUsernameLen {
		writeError(w, http.StatusBadRequest, "username is too long")
		return
	}
	tok, _ := s.Auth.Mint(req.Username)
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: tok, TokenType: "Bearer", Username: req.Username,
		ExpiresIn: int64(s.Auth.ttl / time.Second),
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"username": userFrom(r)})
}

type mintRequest struct {
	Prefix string `json:"prefix"`
	Count  int    `json:"count"`
}

const maxMintCount = 100000

// mintTokens returns a CSV credential pool: username,token for <prefix>1..<prefix>N.
func (s *Server) mintTokens(w http.ResponseWriter, r *http.Request) {
	var req mintRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Prefix == "" {
		req.Prefix = "user"
	}
	if strings.ContainsAny(req.Prefix, ",\"\r\n") || len(req.Prefix) > 64 {
		writeError(w, http.StatusBadRequest, "prefix must be at most 64 characters without commas, quotes or newlines")
		return
	}
	if req.Count < 1 || req.Count > maxMintCount {
		writeError(w, http.StatusBadRequest, "count must be between 1 and 100000")
		return
	}
	var b strings.Builder
	b.Grow(req.Count * 120)
	b.WriteString("username,token\n")
	for i := 1; i <= req.Count; i++ {
		user := req.Prefix + strconv.Itoa(i)
		tok, _ := s.Auth.Mint(user)
		b.WriteString(user)
		b.WriteByte(',')
		b.WriteString(tok)
		b.WriteByte('\n')
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="tokens.csv"`)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(b.String()))
}

// ---- catalogue ----

const (
	defaultLimit = 20
	maxLimit     = 100
)

var validSorts = map[string]bool{"": true, "id": true, "price_asc": true, "price_desc": true, "rating": true, "name": true}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	q := ListQuery{Limit: defaultLimit, Category: qs.Get("category"), Q: strings.TrimSpace(qs.Get("q")), Sort: qs.Get("sort")}
	if v := qs.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		q.Limit = min(n, maxLimit)
	}
	if v := qs.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		q.Offset = n
	}
	if !validSorts[q.Sort] {
		writeError(w, http.StatusBadRequest, "sort must be one of id, price_asc, price_desc, rating, name")
		return
	}
	if q.Sort == "id" {
		q.Sort = ""
	}
	if len(q.Q) > 100 {
		q.Q = q.Q[:100]
	}
	serveBody(w, r, s.Catalog.ListJSON(q), "application/json", "public, max-age=60")
}

// productFromPath resolves {name} as a numeric id or, failing that, a slug.
func (s *Server) productFromPath(w http.ResponseWriter, r *http.Request, name string) *Product {
	v := r.PathValue(name)
	var p *Product
	if id, err := strconv.Atoi(v); err == nil {
		p = s.Catalog.Get(id)
	} else {
		p = s.Catalog.GetBySlug(v)
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "product not found")
	}
	return p
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) {
	if p := s.productFromPath(w, r, "id"); p != nil {
		serveBody(w, r, s.Catalog.detailJSON[p.ID-1], "application/json", "public, max-age=60")
	}
}

func (s *Server) productImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	p := s.Catalog.Get(id)
	if err != nil || p == nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	serveBody(w, r, s.Catalog.svg[p.ID-1], "image/svg+xml", "public, max-age=31536000, immutable")
}

func (s *Server) categories(w http.ResponseWriter, r *http.Request) {
	serveBody(w, r, s.Catalog.categoriesJSON, "application/json", "public, max-age=300")
}

// ---- cart ----

const maxRequestQuantity = 100

type addItemRequest struct {
	ProductID int  `json:"productId"`
	Quantity  *int `json:"quantity"`
}

type quantityRequest struct {
	Quantity *int `json:"quantity"`
}

func (s *Server) getCart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Store.GetCart(userFrom(r)))
}

func (s *Server) addCartItem(w http.ResponseWriter, r *http.Request) {
	var req addItemRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if s.Catalog.Get(req.ProductID) == nil {
		writeError(w, http.StatusBadRequest, "unknown product")
		return
	}
	qty := 1
	if req.Quantity != nil {
		qty = *req.Quantity
	}
	if qty < 1 || qty > maxRequestQuantity {
		writeError(w, http.StatusBadRequest, "quantity must be between 1 and 100")
		return
	}
	writeJSON(w, http.StatusOK, s.Store.AddItem(userFrom(r), req.ProductID, qty))
}

func (s *Server) cartProductID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("productId"))
	if err != nil || s.Catalog.Get(id) == nil {
		writeError(w, http.StatusBadRequest, "unknown product")
		return 0, false
	}
	return id, true
}

func (s *Server) updateCartItem(w http.ResponseWriter, r *http.Request) {
	id, ok := s.cartProductID(w, r)
	if !ok {
		return
	}
	var req quantityRequest
	if !decodeBody(w, r, &req) {
		return
	}
	// PATCH takes the same ceiling a line can accumulate to through POST, so any
	// quantity the cart can show can also be set.
	if req.Quantity == nil || *req.Quantity < 0 || *req.Quantity > maxLineQuantity {
		writeError(w, http.StatusBadRequest, "quantity must be between 0 and 999")
		return
	}
	cart, err := s.Store.SetQuantity(userFrom(r), id, *req.Quantity)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (s *Server) removeCartItem(w http.ResponseWriter, r *http.Request) {
	id, ok := s.cartProductID(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.Store.RemoveItem(userFrom(r), id))
}

// ---- orders ----

type checkoutRequest struct {
	Shipping Shipping `json:"shipping"`
	Payment  struct {
		CardNumber string `json:"cardNumber"`
	} `json:"payment"`
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	var req checkoutRequest
	if !decodeBody(w, r, &req) {
		return
	}
	sh := &req.Shipping
	for _, f := range []*string{&sh.Name, &sh.Email, &sh.Address, &sh.City, &sh.Zip, &sh.Country} {
		*f = strings.TrimSpace(*f)
		if len(*f) > 200 {
			*f = (*f)[:200]
		}
	}
	if sh.Name == "" || sh.Address == "" || !strings.Contains(sh.Email, "@") {
		writeError(w, http.StatusBadRequest, "shipping name, address and a valid email are required")
		return
	}
	digits := make([]byte, 0, 19)
	for _, c := range []byte(req.Payment.CardNumber) {
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	last4 := "0000"
	if len(digits) >= 4 {
		last4 = string(digits[len(digits)-4:])
	}
	o, err := s.Store.Checkout(userFrom(r), *sh, last4)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	orders := s.Store.Orders(userFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{"items": orders, "total": len(orders)})
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	o, err := s.Store.Order(userFrom(r), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) reset(w http.ResponseWriter, r *http.Request) {
	s.Store.Reset()
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
