// Command loadgen is a tiny closed-loop HTTP load client for loadshop.
//
// Each of -c clients logs in as its own user (bearer token from
// POST /api/auth/tokens) and alternates between GET /api/products?limit=50
// and GET /api/products/{random id} for -d, then prints throughput and
// latency percentiles.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type result struct {
	lat    []time.Duration
	status map[int]int
	errs   int
	bytes  int64
}

func main() {
	base := flag.String("url", "http://localhost:8000", "loadshop base URL")
	clients := flag.Int("c", 100, "concurrent clients")
	dur := flag.Duration("d", 10*time.Second, "test duration")
	products := flag.Int("products", 48, "number of products (ids 1..N)")
	cartPct := flag.Int("cart", 0, "percent of iterations that also add to cart (0-100)")
	flag.Parse()

	tokens := mintTokens(*base, *clients)
	tr := &http.Transport{MaxIdleConns: *clients * 2, MaxIdleConnsPerHost: *clients * 2, IdleConnTimeout: 90 * time.Second}
	hc := &http.Client{Transport: tr, Timeout: 30 * time.Second}

	results := make([]result, *clients)
	deadline := time.Now().Add(*dur)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := &results[i]
			r.status = map[int]int{}
			rng := rand.New(rand.NewPCG(uint64(i), 42))
			auth := "Bearer " + tokens[i]
			for n := 0; time.Now().Before(deadline); n++ {
				var req *http.Request
				switch {
				case *cartPct > 0 && rng.IntN(100) < *cartPct:
					body, _ := json.Marshal(map[string]int{"productId": rng.IntN(*products) + 1, "quantity": 1})
					req, _ = http.NewRequest("POST", *base+"/api/cart/items", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
				case n%2 == 0:
					req, _ = http.NewRequest("GET", *base+"/api/products?limit=50", nil)
				default:
					req, _ = http.NewRequest("GET", fmt.Sprintf("%s/api/products/%d", *base, rng.IntN(*products)+1), nil)
				}
				req.Header.Set("Authorization", auth)
				req.Header.Set("Accept-Encoding", "gzip")
				t0 := time.Now()
				resp, err := hc.Do(req)
				if err != nil {
					r.errs++
					continue
				}
				nb, _ := io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				r.lat = append(r.lat, time.Since(t0))
				r.status[resp.StatusCode]++
				r.bytes += nb
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var all []time.Duration
	status := map[int]int{}
	errs := 0
	var bytesTotal int64
	for _, r := range results {
		all = append(all, r.lat...)
		for k, v := range r.status {
			status[k] += v
		}
		errs += r.errs
		bytesTotal += r.bytes
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	pct := func(p float64) time.Duration {
		if len(all) == 0 {
			return 0
		}
		return all[min(len(all)-1, int(float64(len(all))*p))]
	}
	fmt.Printf("clients=%d duration=%s requests=%d errors=%d\n", *clients, elapsed.Round(time.Millisecond), len(all), errs)
	fmt.Printf("throughput: %.0f req/s, %.1f MB/s (wire bytes)\n", float64(len(all))/elapsed.Seconds(), float64(bytesTotal)/elapsed.Seconds()/1e6)
	fmt.Printf("latency: p50=%s p90=%s p99=%s max=%s\n", pct(0.50), pct(0.90), pct(0.99), pct(1))
	fmt.Printf("status: %v\n", status)
	if errs > 0 || status[200] != len(all) {
		os.Exit(1)
	}
}

func mintTokens(base string, n int) []string {
	body, _ := json.Marshal(map[string]any{"prefix": "loadgen", "count": n})
	resp, err := http.Post(base+"/api/auth/tokens", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("mint tokens: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		log.Fatalf("mint tokens: %s %s", resp.Status, b)
	}
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil || len(rows) != n+1 {
		log.Fatalf("mint tokens: bad CSV (%v, %d rows)", err, len(rows))
	}
	out := make([]string, n)
	for i, row := range rows[1:] {
		out[i] = row[1]
	}
	return out
}
