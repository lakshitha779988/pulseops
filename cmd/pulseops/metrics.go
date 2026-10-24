package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// A tiny dependency-free Prometheus histogram for our fixed set of routes.
// Fixed labels avoid high-cardinality production metrics.
var latencyBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5}

type metricKey struct {
	route  string
	status int
}

type requestStat struct {
	count        uint64
	secondsTotal float64
	buckets      [7]uint64 // cumulative buckets
}

type metricStore struct {
	mu       sync.RWMutex
	requests map[metricKey]*requestStat
}

func newMetricStore() *metricStore {
	return &metricStore{requests: make(map[metricKey]*requestStat)}
}

func (m *metricStore) observe(route string, status int, seconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := metricKey{route: route, status: status}
	s := m.requests[key]
	if s == nil {
		s = &requestStat{}
		m.requests[key] = s
	}
	s.count++
	s.secondsTotal += seconds
	for i, upper := range latencyBuckets {
		if seconds <= upper {
			s.buckets[i]++
		}
	}
}

func (m *metricStore) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintln(w, "# HELP pulseops_http_requests_total Total requests to the demo work endpoint.")
	fmt.Fprintln(w, "# TYPE pulseops_http_requests_total counter")

	keys := make([]metricKey, 0, len(m.requests))
	for k := range m.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].status < keys[j].status
	})
	for _, k := range keys {
		s := m.requests[k]
		fmt.Fprintf(w, "pulseops_http_requests_total{route=%q,status=%q} %d\n", k.route, strconv.Itoa(k.status), s.count)
	}

	fmt.Fprintln(w, "# HELP pulseops_http_request_duration_seconds Duration of demo work requests in seconds.")
	fmt.Fprintln(w, "# TYPE pulseops_http_request_duration_seconds histogram")
	for _, k := range keys {
		s := m.requests[k]
		prefix := fmt.Sprintf("route=%q,status=%q", k.route, strconv.Itoa(k.status))
		for i, upper := range latencyBuckets {
			// str.FormatFloat produces the bucket boundary for Prometheus le labels.
			le := strings.TrimSpace(strconv.FormatFloat(upper, 'f', -1, 64))
			fmt.Fprintf(w, "pulseops_http_request_duration_seconds_bucket{%s,le=%q} %d\n", prefix, le, s.buckets[i])
		}
		fmt.Fprintf(w, "pulseops_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", prefix, s.count)
		fmt.Fprintf(w, "pulseops_http_request_duration_seconds_sum{%s} %g\n", prefix, s.secondsTotal)
		fmt.Fprintf(w, "pulseops_http_request_duration_seconds_count{%s} %d\n", prefix, s.count)
	}
}
