// Package perf provides the Phase 15 performance measurement and
// load-generation harness.
//
// WHY A GO HARNESS RATHER THAN k6/wrk/ab: none of those tools is
// installed in this environment (verified — k6, hey, ab, wrk, vegeta and
// siege are all absent, and psql is too). The phase brief says to use an
// appropriate tool already available rather than requiring a new
// platform, so this harness uses Go's own concurrency primitives to
// drive the real gin route table in-process via httptest. That is a real
// measurement of handler + database time under real concurrency; what it
// does NOT measure is network stack, TLS termination, or load-balancer
// behaviour, which an out-of-process tool would. Every number this
// produces is reported with that limitation attached.
//
// NOTHING HERE RUNS IN PRODUCTION. This package is only invoked from
// tests (perf_test.go) and seeds its own isolated, clearly-marked test
// data which it removes afterwards.
package perf

import (
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Result is one load-test measurement. Field names match the evidence
// table the phase brief requires (item AL).
type Result struct {
	Name            string
	ConcurrentUsers int
	Duration        time.Duration
	Requests        int64
	Errors          int64
	RequestsPerSec  float64
	P50, P95, P99   time.Duration
	Min, Max        time.Duration
	ErrorRatePct    float64
	AllocMiB        float64 // heap allocated at end of run
	Goroutines      int
}

// RequestFunc builds one request to issue. It is called per iteration so
// a scenario can vary its target (e.g. paginate, or hit different IDs).
type RequestFunc func(i int) *http.Request

// Run drives fn at the given concurrency for the given duration against
// the supplied engine, recording per-request latency.
//
// Latency is measured around engine.ServeHTTP only — it excludes request
// construction, so the number reflects application+DB time rather than
// harness overhead.
func Run(name string, engine *gin.Engine, concurrency int, duration time.Duration, fn RequestFunc) Result {
	var (
		mu        sync.Mutex
		latencies []time.Duration
		requests  atomic.Int64
		errors    atomic.Int64
		wg        sync.WaitGroup
	)

	deadline := time.Now().Add(duration)
	start := time.Now()

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			local := make([]time.Duration, 0, 256)
			i := worker
			for time.Now().Before(deadline) {
				req := fn(i)
				rec := httptest.NewRecorder()

				t0 := time.Now()
				engine.ServeHTTP(rec, req)
				elapsed := time.Since(t0)

				local = append(local, elapsed)
				requests.Add(1)
				// 5xx is a server failure. 4xx is not counted as an error
				// here: several scenarios deliberately exercise
				// authorization rejections, and counting those as
				// failures would misreport a working control as an error
				// rate.
				if rec.Code >= 500 {
					errors.Add(1)
				}
				i += concurrency
			}
			mu.Lock()
			latencies = append(latencies, local...)
			mu.Unlock()
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	return summarize(name, concurrency, elapsed, requests.Load(), errors.Load(), latencies, &ms)
}

func summarize(name string, concurrency int, elapsed time.Duration, reqs, errs int64, latencies []time.Duration, ms *runtime.MemStats) Result {
	r := Result{
		Name: name, ConcurrentUsers: concurrency, Duration: elapsed,
		Requests: reqs, Errors: errs,
		Goroutines: runtime.NumGoroutine(),
		AllocMiB:   float64(ms.HeapAlloc) / 1024 / 1024,
	}
	if elapsed > 0 {
		r.RequestsPerSec = float64(reqs) / elapsed.Seconds()
	}
	if reqs > 0 {
		r.ErrorRatePct = float64(errs) / float64(reqs) * 100
	}
	if len(latencies) == 0 {
		return r
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	r.Min = latencies[0]
	r.Max = latencies[len(latencies)-1]
	r.P50 = percentile(latencies, 50)
	r.P95 = percentile(latencies, 95)
	r.P99 = percentile(latencies, 99)
	return r
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
