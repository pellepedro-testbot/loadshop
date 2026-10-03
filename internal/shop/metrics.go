package shop

import (
	"fmt"
	"io"
	"sort"
	"sync/atomic"
	"time"
)

// routeCounters counts responses by status class (index 1..5 => 1xx..5xx, 0 => other).
type routeCounters [6]atomic.Int64

// Metrics are lock-free counters. The route map is built before serving
// starts and is read-only afterwards, so lookups need no locking.
type Metrics struct {
	start    time.Time
	total    atomic.Int64
	inFlight atomic.Int64
	routes   map[string]*routeCounters
	other    routeCounters
}

func newMetrics() *Metrics {
	return &Metrics{start: time.Now(), routes: map[string]*routeCounters{}}
}

func (m *Metrics) register(pattern string) { m.routes[pattern] = &routeCounters{} }

func (m *Metrics) record(pattern string, status int) {
	rc := m.routes[pattern]
	if rc == nil {
		rc = &m.other
	}
	class := status / 100
	if class < 1 || class > 5 {
		class = 0
	}
	rc[class].Add(1)
}

func (m *Metrics) write(w io.Writer) {
	fmt.Fprintf(w, "# loadshop metrics\n")
	fmt.Fprintf(w, "uptime_seconds %.0f\n", time.Since(m.start).Seconds())
	fmt.Fprintf(w, "requests_total %d\n", m.total.Load())
	fmt.Fprintf(w, "requests_in_flight %d\n", m.inFlight.Load())
	patterns := make([]string, 0, len(m.routes))
	for p := range m.routes {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)
	classes := []string{"other", "1xx", "2xx", "3xx", "4xx", "5xx"}
	emit := func(name string, rc *routeCounters) {
		for i := range rc {
			if n := rc[i].Load(); n > 0 {
				fmt.Fprintf(w, "requests{route=%q,status=%q} %d\n", name, classes[i], n)
			}
		}
	}
	for _, p := range patterns {
		emit(p, m.routes[p])
	}
	emit("unmatched", &m.other)
}
