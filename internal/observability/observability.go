// Package observability implements the Phase 16 metrics and request-
// correlation foundation (items T, U, AD), both of which Phase 15
// recorded as MISSING.
//
// DELIBERATELY VENDOR-NEUTRAL. The phase brief says not to require a
// specific monitoring vendor, so this exposes an in-process registry and
// a plain-JSON snapshot endpoint. Anything that can scrape JSON — a
// Prometheus json_exporter, a Datadog HTTP check, a cron + curl — can
// consume it, and no third-party client library is added to go.mod.
//
// SCOPE LIMIT, stated plainly: metrics are held in memory in a single
// process. They reset on restart and are NOT aggregated across
// instances. This is a foundation for monitoring, not a substitute for a
// real metrics backend — see the Phase 16 report's open risks.
package observability

import (
	"sort"
	"sync"
	"time"
)

// Counter names are plain strings so a caller can record a new event
// type without editing this package. Convention: dot-separated,
// lowercase, e.g. "http.requests", "payment.webhook.failed".
const (
	MetricHTTPRequests    = "http.requests"
	MetricHTTPErrors5xx   = "http.errors.5xx"
	MetricHTTPErrors4xx   = "http.errors.4xx"
	MetricAuthFailures    = "auth.failures"
	MetricRateLimitHits   = "ratelimit.violations"
	MetricPaymentFailures = "payment.failures"
	MetricWebhookFailures = "webhook.failures"
	MetricExportFailures  = "export.failures"
	MetricExportRejected  = "export.rejected"
	MetricJobFailures     = "job.failures"
	MetricIntegrityFail   = "integrity.check.failed"
)

var registry = newRegistry()

type latencySeries struct {
	// Reservoir of recent samples. Bounded so a long-running process
	// cannot grow this without limit — the whole point of the package is
	// to observe leaks, not cause one.
	samples []time.Duration
	next    int
	full    bool
}

const latencyReservoir = 2048

type metricRegistry struct {
	mu        sync.RWMutex
	counters  map[string]int64
	gauges    map[string]float64
	latencies map[string]*latencySeries
	startedAt time.Time
}

func newRegistry() *metricRegistry {
	return &metricRegistry{
		counters:  map[string]int64{},
		gauges:    map[string]float64{},
		latencies: map[string]*latencySeries{},
		startedAt: time.Now(),
	}
}

// Inc increments a named counter.
func Inc(name string) { Add(name, 1) }

// Add adds n to a named counter.
func Add(name string, n int64) {
	registry.mu.Lock()
	registry.counters[name] += n
	registry.mu.Unlock()
}

// SetGauge records a point-in-time value (connection counts, queue
// depth) that can go up or down.
func SetGauge(name string, v float64) {
	registry.mu.Lock()
	registry.gauges[name] = v
	registry.mu.Unlock()
}

// ObserveLatency records one duration sample under a name.
func ObserveLatency(name string, d time.Duration) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	s, ok := registry.latencies[name]
	if !ok {
		s = &latencySeries{samples: make([]time.Duration, latencyReservoir)}
		registry.latencies[name] = s
	}
	s.samples[s.next] = d
	s.next = (s.next + 1) % latencyReservoir
	if s.next == 0 {
		s.full = true
	}
}

// LatencySummary is the percentile view of one latency series.
type LatencySummary struct {
	Count int     `json:"count"`
	P50Ms float64 `json:"p50Ms"`
	P95Ms float64 `json:"p95Ms"`
	P99Ms float64 `json:"p99Ms"`
	MaxMs float64 `json:"maxMs"`
}

// Snapshot is the full metrics view returned by the metrics endpoint.
type Snapshot struct {
	UptimeSeconds float64                   `json:"uptimeSeconds"`
	Counters      map[string]int64          `json:"counters"`
	Gauges        map[string]float64        `json:"gauges"`
	Latency       map[string]LatencySummary `json:"latency"`
	CollectedAt   time.Time                 `json:"collectedAt"`
	// Note travels with the payload so a consumer cannot mistake
	// single-process in-memory metrics for cluster-wide truth.
	Note string `json:"note"`
}

func Collect() Snapshot {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	counters := make(map[string]int64, len(registry.counters))
	for k, v := range registry.counters {
		counters[k] = v
	}
	gauges := make(map[string]float64, len(registry.gauges))
	for k, v := range registry.gauges {
		gauges[k] = v
	}
	lat := make(map[string]LatencySummary, len(registry.latencies))
	for k, s := range registry.latencies {
		lat[k] = s.summary()
	}

	return Snapshot{
		UptimeSeconds: time.Since(registry.startedAt).Seconds(),
		Counters:      counters,
		Gauges:        gauges,
		Latency:       lat,
		CollectedAt:   time.Now(),
		Note:          "In-memory, single-process metrics. Values reset on restart and are NOT aggregated across instances.",
	}
}

func (s *latencySeries) summary() LatencySummary {
	n := s.next
	if s.full {
		n = len(s.samples)
	}
	if n == 0 {
		return LatencySummary{}
	}
	cp := make([]time.Duration, n)
	copy(cp, s.samples[:n])
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })

	ms := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }
	pick := func(p float64) time.Duration {
		idx := int(p/100*float64(n)) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		return cp[idx]
	}
	return LatencySummary{
		Count: n,
		P50Ms: ms(pick(50)), P95Ms: ms(pick(95)), P99Ms: ms(pick(99)),
		MaxMs: ms(cp[n-1]),
	}
}

// Reset clears all metrics. Test-only helper — never called by the
// application, since resetting counters in production would destroy the
// very history an operator is trying to read.
func Reset() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.counters = map[string]int64{}
	registry.gauges = map[string]float64{}
	registry.latencies = map[string]*latencySeries{}
	registry.startedAt = time.Now()
}
