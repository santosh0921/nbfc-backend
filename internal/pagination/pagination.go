// Package pagination provides the shared, bounded list-paging contract
// for admin list endpoints (Phase 15 item F).
//
// The rule this package exists to enforce: no production list endpoint
// may return an unbounded dataset. Before Phase 15 several list handlers
// issued a bare Find() over a table that grows with the customer base —
// harmless at demo scale, fatal at 100,000 customers.
//
// Limits are deliberately conservative and capped server-side: a client
// asking for ?limit=1000000 gets MaxLimit, not a million rows, because
// the cap exists to protect the server, not to be advisory.
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultLimit is what a client gets when it asks for no particular
	// page size — small enough that an admin screen loads fast on a
	// large table.
	DefaultLimit = 25

	// MaxLimit is the hard server-side ceiling. A client cannot exceed
	// it regardless of what it asks for. 200 keeps the heaviest
	// batch-loaded list handler's per-request work bounded while still
	// allowing a deliberately larger page for export-style views.
	MaxLimit = 200
)

// Params is one validated page request.
type Params struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// FromQuery reads ?page= and ?limit=, clamping both into safe ranges.
// Invalid, negative, or absent values fall back to defaults rather than
// erroring — a malformed paging hint should not fail an otherwise valid
// request, it should just page conservatively.
func FromQuery(c *gin.Context) Params {
	p := Params{Page: 1, Limit: DefaultLimit}

	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Page = n
		}
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Limit = n
		}
	}
	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}
	return p
}

// Offset is the SQL OFFSET for this page.
//
// NOTE ON DEEP PAGINATION: offset paging makes the database skip every
// preceding row, so page 10,000 is materially more expensive than page 1
// on a large table. That is acceptable for admin screens, which are
// browsed from the front and filtered rather than paged into the deep
// tail. An endpoint that genuinely needs to walk a whole large table
// should use keyset/cursor paging on an indexed column instead — see the
// Phase 15 report's remaining-risks section.
func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

// Meta is the pagination block returned alongside a page of results.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Count      int   `json:"count"`      // rows in THIS response
	Total      int64 `json:"total"`      // rows in the full result set
	TotalPages int64 `json:"totalPages"`
	HasNext    bool  `json:"hasNext"`
	HasPrev    bool  `json:"hasPrev"`
	MaxLimit   int   `json:"maxLimit"` // advertised so clients know the ceiling
}

func (p Params) Meta(total int64, count int) Meta {
	totalPages := int64(0)
	if p.Limit > 0 {
		totalPages = (total + int64(p.Limit) - 1) / int64(p.Limit)
	}
	return Meta{
		Page: p.Page, Limit: p.Limit, Count: count, Total: total,
		TotalPages: totalPages,
		HasNext:    int64(p.Page) < totalPages,
		HasPrev:    p.Page > 1,
		MaxLimit:   MaxLimit,
	}
}
