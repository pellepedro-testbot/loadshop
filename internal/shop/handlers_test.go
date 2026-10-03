package shop

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	ui := fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><div id=root></div>")},
		"assets/app-1.js": {Data: []byte(strings.Repeat("console.log(1);", 200))},
	}
	s, err := New(Config{Secret: "test-secret", TokenTTL: time.Hour, UI: ui})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func do(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		data, _ := json.Marshal(b)
		rd = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func login(t *testing.T, s *Server, user string) string {
	t.Helper()
	rec := do(t, s, "POST", "/api/auth/login", "", map[string]string{"username": user, "password": "whatever"})
	if rec.Code != 200 {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	return decode[loginResponse](t, rec).AccessToken
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, code, rec.Body)
	}
}

func TestLoginAcceptsAnyCredentials(t *testing.T) {
	s := newTestServer(t)
	for _, u := range []string{"alice", "bob@example.com", "user 42", "x"} {
		rec := do(t, s, "POST", "/api/auth/login", "", map[string]string{"username": u, "password": "p" + u})
		wantStatus(t, rec, 200)
		resp := decode[loginResponse](t, rec)
		if resp.Username != u || resp.AccessToken == "" || resp.ExpiresIn != 3600 {
			t.Fatalf("bad login response %+v", resp)
		}
		me := do(t, s, "GET", "/api/auth/me", resp.AccessToken, nil)
		wantStatus(t, me, 200)
		if got := decode[map[string]string](t, me)["username"]; got != u {
			t.Fatalf("me = %q, want %q", got, u)
		}
	}
	wantStatus(t, do(t, s, "POST", "/api/auth/login", "", map[string]string{"username": "", "password": "x"}), 400)
	wantStatus(t, do(t, s, "POST", "/api/auth/login", "", map[string]string{"username": "a", "password": ""}), 400)
	wantStatus(t, do(t, s, "POST", "/api/auth/login", "", "{not json"), 400)
}

func TestTokenValidation(t *testing.T) {
	s := newTestServer(t)
	tok := login(t, s, "alice")
	wantStatus(t, do(t, s, "GET", "/api/cart", "", nil), 401)
	wantStatus(t, do(t, s, "GET", "/api/cart", "garbage", nil), 401)
	wantStatus(t, do(t, s, "GET", "/api/cart", tok+"x", nil), 401)
	// Signed with another secret.
	other, _ := NewAuth("other-secret", time.Hour).Mint("alice")
	wantStatus(t, do(t, s, "GET", "/api/cart", other, nil), 401)
	// Expired.
	a := NewAuth("test-secret", time.Hour)
	a.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expired, _ := a.Mint("alice")
	wantStatus(t, do(t, s, "GET", "/api/cart", expired, nil), 401)
	wantStatus(t, do(t, s, "GET", "/api/cart", tok, nil), 200)
	// Lowercase scheme is accepted.
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "bearer "+tok)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	wantStatus(t, rec, 200)
	// Usernames containing the separator round-trip.
	weird := login(t, s, "a|b")
	if got := decode[map[string]string](t, do(t, s, "GET", "/api/auth/me", weird, nil))["username"]; got != "a|b" {
		t.Fatalf("username = %q", got)
	}
}

func TestMintTokensCSV(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, "POST", "/api/auth/tokens", "", map[string]any{"prefix": "vu", "count": 5})
	wantStatus(t, rec, 200)
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type %q", ct)
	}
	rows, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 || rows[0][0] != "username" || rows[0][1] != "token" {
		t.Fatalf("rows = %v", rows)
	}
	for i, row := range rows[1:] {
		want := fmt.Sprintf("vu%d", i+1)
		if row[0] != want {
			t.Fatalf("row %d user %q", i, row[0])
		}
		if got := decode[map[string]string](t, do(t, s, "GET", "/api/auth/me", row[1], nil))["username"]; got != want {
			t.Fatalf("token for %s resolves to %q", want, got)
		}
	}
	wantStatus(t, do(t, s, "POST", "/api/auth/tokens", "", map[string]any{"prefix": "vu", "count": 0}), 400)
}

type listResp struct {
	Items []Product `json:"items"`
	Total int       `json:"total"`
}

func TestCatalogue(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, "GET", "/api/products?limit=50", "", nil)
	wantStatus(t, rec, 200)
	all := decode[listResp](t, rec)
	if all.Total != 48 || len(all.Items) != 48 {
		t.Fatalf("total=%d items=%d", all.Total, len(all.Items))
	}
	names := map[string]bool{}
	for i, p := range all.Items {
		if p.ID != i+1 {
			t.Fatalf("item %d has id %d", i, p.ID)
		}
		names[p.Name] = true
	}
	for _, n := range []string{"Google Pixel 4", "Apple MacBook Air"} {
		if !names[n] {
			t.Fatalf("missing %q", n)
		}
	}

	// Paging.
	page := decode[listResp](t, do(t, s, "GET", "/api/products?limit=10&offset=20", "", nil))
	if page.Total != 48 || len(page.Items) != 10 || page.Items[0].ID != 21 {
		t.Fatalf("page: total=%d len=%d first=%d", page.Total, len(page.Items), page.Items[0].ID)
	}
	tail := decode[listResp](t, do(t, s, "GET", "/api/products?limit=10&offset=45", "", nil))
	if len(tail.Items) != 3 {
		t.Fatalf("tail len %d", len(tail.Items))
	}
	past := decode[listResp](t, do(t, s, "GET", "/api/products?offset=500", "", nil))
	if len(past.Items) != 0 || past.Total != 48 {
		t.Fatalf("past end: %+v", past)
	}

	// Filters and sort.
	phones := decode[listResp](t, do(t, s, "GET", "/api/products?category=phones&limit=100", "", nil))
	if phones.Total != 8 {
		t.Fatalf("phones total %d", phones.Total)
	}
	pixel := decode[listResp](t, do(t, s, "GET", "/api/products?q=pixel%204", "", nil))
	if pixel.Total != 1 || pixel.Items[0].Name != "Google Pixel 4" {
		t.Fatalf("search: %+v", pixel)
	}
	asc := decode[listResp](t, do(t, s, "GET", "/api/products?sort=price_asc&limit=100", "", nil))
	for i := 1; i < len(asc.Items); i++ {
		if asc.Items[i].Price < asc.Items[i-1].Price {
			t.Fatal("not sorted by price")
		}
	}
	wantStatus(t, do(t, s, "GET", "/api/products?limit=abc", "", nil), 400)
	wantStatus(t, do(t, s, "GET", "/api/products?sort=bogus", "", nil), 400)

	// Detail.
	rec = do(t, s, "GET", "/api/products/22", "", nil)
	wantStatus(t, rec, 200)
	p := decode[Product](t, rec)
	if p.ID != 22 || p.Name != all.Items[21].Name || len(p.Specs) == 0 || p.Description == "" {
		t.Fatalf("detail: %+v", p)
	}
	wantStatus(t, do(t, s, "GET", "/api/products/0", "", nil), 404)
	wantStatus(t, do(t, s, "GET", "/api/products/999", "", nil), 404)
	wantStatus(t, do(t, s, "GET", "/api/products/abc", "", nil), 404)
	if bySlug := decode[Product](t, do(t, s, "GET", "/api/products/google-pixel-4", "", nil)); bySlug.Name != "Google Pixel 4" {
		t.Fatalf("slug lookup: %+v", bySlug)
	}

	cats := decode[[]Category](t, do(t, s, "GET", "/api/categories", "", nil))
	if len(cats) != 6 {
		t.Fatalf("categories %v", cats)
	}

	img := do(t, s, "GET", "/api/products/9/image.svg", "", nil)
	wantStatus(t, img, 200)
	if img.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(img.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("image headers %v", img.Header())
	}
}

func TestCartFlow(t *testing.T) {
	s := newTestServer(t)
	tok := login(t, s, "carol")

	cart := decode[Cart](t, do(t, s, "GET", "/api/cart", tok, nil))
	if len(cart.Items) != 0 {
		t.Fatalf("new cart not empty")
	}
	rec := do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 1, "quantity": 2})
	wantStatus(t, rec, 200)
	do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 1, "quantity": 1})
	cart = decode[Cart](t, do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 9}))
	if len(cart.Items) != 2 || cart.Items[0].Quantity != 3 || cart.Items[1].Quantity != 1 || cart.ItemCount != 4 {
		t.Fatalf("cart after adds: %+v", cart)
	}
	p1, p9 := s.Catalog.Get(1), s.Catalog.Get(9)
	wantSub := (p1.priceCents*3 + p9.priceCents) / 1
	if int64(cart.Subtotal*100+0.5) != wantSub {
		t.Fatalf("subtotal %v want %d cents", cart.Subtotal, wantSub)
	}

	cart = decode[Cart](t, do(t, s, "PATCH", "/api/cart/items/1", tok, map[string]int{"quantity": 5}))
	if cart.Items[0].Quantity != 5 {
		t.Fatalf("patch: %+v", cart.Items)
	}
	cart = decode[Cart](t, do(t, s, "DELETE", "/api/cart/items/9", tok, nil))
	if len(cart.Items) != 1 {
		t.Fatalf("delete: %+v", cart.Items)
	}
	// Above the POST per-request limit but within the line cap.
	if c := decode[Cart](t, do(t, s, "PATCH", "/api/cart/items/1", tok, map[string]int{"quantity": 150})); c.Items[0].Quantity != 150 {
		t.Fatalf("patch to 150: %+v", c)
	}
	cart = decode[Cart](t, do(t, s, "PATCH", "/api/cart/items/1", tok, map[string]int{"quantity": 0}))
	if len(cart.Items) != 0 || cart.Total != 0 {
		t.Fatalf("patch to zero: %+v", cart)
	}

	// Validation.
	wantStatus(t, do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 999, "quantity": 1}), 400)
	wantStatus(t, do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 1, "quantity": 0}), 400)
	wantStatus(t, do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 1, "quantity": -3}), 400)
	wantStatus(t, do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 1, "quantity": 101}), 400)
	wantStatus(t, do(t, s, "PATCH", "/api/cart/items/1", tok, map[string]int{"quantity": 1000}), 400)
	wantStatus(t, do(t, s, "PATCH", "/api/cart/items/2", tok, map[string]int{"quantity": 1}), 404)
	wantStatus(t, do(t, s, "PATCH", "/api/cart/items/abc", tok, map[string]int{"quantity": 1}), 400)
	wantStatus(t, do(t, s, "PATCH", "/api/cart/items/1", tok, map[string]string{}), 400)

	// Carts are per user.
	other := login(t, s, "dave")
	do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 3, "quantity": 1})
	if c := decode[Cart](t, do(t, s, "GET", "/api/cart", other, nil)); len(c.Items) != 0 {
		t.Fatalf("cart leaked across users: %+v", c)
	}
}

var shipping = map[string]any{
	"shipping": map[string]string{"name": "Ada Lovelace", "email": "ada@example.com", "address": "1 Analytical St", "city": "London", "zip": "N1", "country": "UK"},
	"payment":  map[string]string{"cardNumber": "4242 4242 4242 4242"},
}

func TestCheckoutAndOrders(t *testing.T) {
	s := newTestServer(t)
	tok := login(t, s, "erin")
	wantStatus(t, do(t, s, "POST", "/api/orders", tok, shipping), 400) // empty cart

	do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 2, "quantity": 2})
	wantStatus(t, do(t, s, "POST", "/api/orders", tok, map[string]any{"shipping": map[string]string{"name": "x"}}), 400)

	rec := do(t, s, "POST", "/api/orders", tok, shipping)
	wantStatus(t, rec, 201)
	o := decode[Order](t, rec)
	if o.ID == "" || o.Status != "confirmed" || len(o.Items) != 1 || o.Items[0].Quantity != 2 || o.PaymentLast4 != "4242" || o.Total <= o.Subtotal {
		t.Fatalf("order: %+v", o)
	}
	if c := decode[Cart](t, do(t, s, "GET", "/api/cart", tok, nil)); len(c.Items) != 0 {
		t.Fatal("cart not emptied by checkout")
	}

	list := decode[struct {
		Items []Order `json:"items"`
		Total int     `json:"total"`
	}](t, do(t, s, "GET", "/api/orders", tok, nil))
	if list.Total != 1 || list.Items[0].ID != o.ID {
		t.Fatalf("orders: %+v", list)
	}
	got := decode[Order](t, do(t, s, "GET", "/api/orders/"+o.ID, tok, nil))
	if got.ID != o.ID {
		t.Fatalf("get order %+v", got)
	}
	// Another user cannot see it.
	wantStatus(t, do(t, s, "GET", "/api/orders/"+o.ID, login(t, s, "mallory"), nil), 404)
}

func TestReset(t *testing.T) {
	s := newTestServer(t)
	tok := login(t, s, "frank")
	do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 1})
	do(t, s, "POST", "/api/orders", tok, shipping)
	do(t, s, "POST", "/api/cart/items", tok, map[string]int{"productId": 2})
	wantStatus(t, do(t, s, "POST", "/api/admin/reset", "", nil), 200)
	if c := decode[Cart](t, do(t, s, "GET", "/api/cart", tok, nil)); len(c.Items) != 0 {
		t.Fatal("cart survived reset")
	}
	if l := decode[map[string]any](t, do(t, s, "GET", "/api/orders", tok, nil)); l["total"].(float64) != 0 {
		t.Fatal("orders survived reset")
	}
	// Tokens stay valid after reset (they are stateless).
	wantStatus(t, do(t, s, "GET", "/api/auth/me", tok, nil), 200)
}

func TestConcurrentCartUpdates(t *testing.T) {
	s := newTestServer(t)
	const users, workers, adds = 20, 10, 25
	tokens := make([]string, users)
	for i := range tokens {
		tokens[i] = login(t, s, fmt.Sprintf("load%d", i))
	}
	shared := login(t, s, "shared")

	var wg sync.WaitGroup
	errs := make(chan string, users*workers+workers)
	for u := 0; u < users; u++ {
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(u, w int) {
				defer wg.Done()
				for i := 0; i < adds; i++ {
					pid := (u+w+i)%48 + 1
					rec := do(t, s, "POST", "/api/cart/items", tokens[u], map[string]int{"productId": pid, "quantity": 1})
					if rec.Code != 200 {
						errs <- rec.Body.String()
						return
					}
					// Mix in reads and catalogue traffic.
					do(t, s, "GET", "/api/cart", tokens[u], nil)
					do(t, s, "GET", fmt.Sprintf("/api/products/%d", pid), "", nil)
				}
			}(u, w)
		}
	}
	// Many goroutines hammering one user's single line.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < adds; i++ {
				if rec := do(t, s, "POST", "/api/cart/items", shared, map[string]int{"productId": 7, "quantity": 1}); rec.Code != 200 {
					errs <- rec.Body.String()
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	for u := 0; u < users; u++ {
		c := decode[Cart](t, do(t, s, "GET", "/api/cart", tokens[u], nil))
		if c.ItemCount != workers*adds {
			t.Fatalf("user %d item count %d, want %d", u, c.ItemCount, workers*adds)
		}
	}
	c := decode[Cart](t, do(t, s, "GET", "/api/cart", shared, nil))
	if len(c.Items) != 1 || c.Items[0].Quantity != workers*adds {
		t.Fatalf("shared cart: %+v", c.Items)
	}
}

func TestStaticAndMisc(t *testing.T) {
	s := newTestServer(t)
	wantStatus(t, do(t, s, "GET", "/healthz", "", nil), 200)

	idx := do(t, s, "GET", "/product/google-pixel-4", "", nil) // SPA fallback
	wantStatus(t, idx, 200)
	if !strings.Contains(idx.Body.String(), "root") || idx.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("fallback: %v %s", idx.Header(), idx.Body)
	}
	req := httptest.NewRequest("GET", "/assets/app-1.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	wantStatus(t, rec, 200)
	if rec.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset headers %v", rec.Header())
	}
	wantStatus(t, do(t, s, "GET", "/assets/missing.js", "", nil), 404)

	nf := do(t, s, "GET", "/api/nope", "", nil)
	wantStatus(t, nf, 404)
	if decode[map[string]string](t, nf)["error"] == "" {
		t.Fatal("expected JSON error")
	}
	pre := do(t, s, "OPTIONS", "/api/cart", "", nil)
	wantStatus(t, pre, 204)
	if pre.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}

	m := do(t, s, "GET", "/metrics", "", nil)
	wantStatus(t, m, 200)
	body := m.Body.String()
	for _, want := range []string{"requests_total", "requests_in_flight", "uptime_seconds", `route="GET /healthz",status="2xx"`, `route="/api/",status="4xx"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q:\n%s", want, body)
		}
	}
}

// Product detail is served with an ETag like the other catalogue bodies, and a full list cache still serves (lazily gzipped) list responses.
func TestDetailAndUncachedListEncoding(t *testing.T) {
	s := newTestServer(t)
	get := func(path string, gz bool, inm string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if gz {
			req.Header.Set("Accept-Encoding", "gzip")
		}
		if inm != "" {
			req.Header.Set("If-None-Match", inm)
		}
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		return rec
	}

	d := get("/api/products/1", true, "")
	wantStatus(t, d, 200)
	etag := d.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("detail headers %v", d.Header())
	}
	wantStatus(t, get("/api/products/1", false, etag), 304)

	s.Catalog.listCacheSize.Store(maxListCacheEntries)
	plain := get("/api/products?q=a&limit=100", false, "")
	wantStatus(t, plain, 200)
	if plain.Header().Get("Content-Encoding") != "" || decode[map[string]any](t, plain)["total"] == nil {
		t.Fatalf("uncached plain list: %v", plain.Header())
	}
	if z := get("/api/products?q=a&limit=100", true, ""); z.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("uncached gzip list: %v", z.Header())
	}
}
