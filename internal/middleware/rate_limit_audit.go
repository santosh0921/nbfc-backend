package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
)

// Rate-limit audit throttling (Phase 16, found during the item-AM soak
// test).
//
// THE PROBLEM: NamedRateLimit wrote one audit row per rejected request,
// synchronously, on the request path. audit.RecordDetailed performs a
// blocking INSERT, so every 429 cost a database write and a pooled
// connection. Under sustained load the soak measured those inserts
// taking multiple SECONDS each.
//
// That is a denial-of-service AMPLIFICATION: the harder an attacker
// floods an endpoint, the more database writes the defence mechanism
// itself generates, competing for the same connection pool that serves
// legitimate traffic — and inflating an append-only table that by design
// cannot be pruned. The control intended to absorb abuse became a
// multiplier for it.
//
// THE FIX: aggregate. The security signal an operator needs is "this IP
// is hammering this endpoint", not one row per rejected packet. At most
// one audit row is written per (IP, endpoint) per window, and it carries
// the number of violations suppressed since the last write — so no
// information about volume is lost, while the write rate becomes bounded
// regardless of attack volume.
//
// Deliberately scoped to rate-limit violations ONLY. Financial actions,
// approvals, exports, and document access continue to write one audit
// row per event, synchronously — those are low-volume and individually
// significant, and weakening them to reduce writes would trade away the
// audit trail this system depends on.

const rateLimitAuditWindow = time.Minute

type violationState struct {
	lastWritten time.Time
	suppressed  int64
}

var (
	rlAuditMu     sync.Mutex
	rlAuditStates = map[string]*violationState{}
	rlAuditSweep  time.Time
)

// recordRateLimitViolation writes an aggregated audit entry, or records
// the violation as suppressed if one was written for this key recently.
func recordRateLimitViolation(c *gin.Context, label, ip, endpoint string) {
	key := ip + "|" + endpoint

	rlAuditMu.Lock()
	now := time.Now()

	// Opportunistic sweep so an attacker cycling source IPs cannot grow
	// this map without bound — the same concern the limiter's own
	// visitor map already handles.
	if now.Sub(rlAuditSweep) > 10*time.Minute {
		for k, v := range rlAuditStates {
			if now.Sub(v.lastWritten) > 30*time.Minute && v.suppressed == 0 {
				delete(rlAuditStates, k)
			}
		}
		rlAuditSweep = now
	}

	st, ok := rlAuditStates[key]
	if !ok {
		st = &violationState{}
		rlAuditStates[key] = st
	}

	// Within the window: count it and return without touching the
	// database at all. This is the hot path under attack.
	if !st.lastWritten.IsZero() && now.Sub(st.lastWritten) < rateLimitAuditWindow {
		st.suppressed++
		rlAuditMu.Unlock()
		return
	}

	suppressed := st.suppressed
	st.suppressed = 0
	st.lastWritten = now
	rlAuditMu.Unlock()

	details := label
	if suppressed > 0 {
		details = fmt.Sprintf("%s (%d further violation(s) from this source suppressed in the preceding window)",
			label, suppressed)
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "unknown", ActorRef: ip,
		Action: "rate_limit.exceeded", TargetType: "endpoint", TargetID: endpoint,
		Result: audit.ResultDenied, Details: details,
	})
}

// ResetRateLimitAuditStateForTest clears the aggregation state so tests
// are independent of one another. Test-only.
func ResetRateLimitAuditStateForTest() {
	rlAuditMu.Lock()
	rlAuditStates = map[string]*violationState{}
	rlAuditSweep = time.Time{}
	rlAuditMu.Unlock()
}
