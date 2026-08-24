package perf_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestGenerateRouteInventory (Phase 16 item AH) enumerates EVERY route
// the production route table registers and writes it to
// PHASE16_ROUTE_INVENTORY.md.
//
// Item AH is explicit that inventory is not the same as security
// testing, so this classifies each route by the authentication tier it
// sits behind — which is derivable from the route table — and does NOT
// claim any route was security-tested. The Phase 16 report keeps
// INVENTORIED / SECURITY-TESTED / NOT-TESTED as three separate counts.
//
// Opt-in via PERF=1 like the other generative tests, since it writes a
// file into the repository.
func TestGenerateRouteInventory(t *testing.T) {
	requirePerf(t)
	r := ensureApp(t)

	routes := r.Routes()
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	var public, customer, employee, adminRoutes []gin.RouteInfo
	for _, rt := range routes {
		switch {
		case strings.HasPrefix(rt.Path, "/admin"):
			// /admin/login and /admin/refresh are public entry points
			// even though they share the /admin prefix.
			if rt.Path == "/admin/login" || rt.Path == "/admin/refresh" {
				public = append(public, rt)
			} else {
				adminRoutes = append(adminRoutes, rt)
			}
		case strings.HasPrefix(rt.Path, "/auth/"):
			if isPublicAuth(rt.Path) {
				public = append(public, rt)
			} else {
				customer = append(customer, rt)
			}
		case strings.HasPrefix(rt.Path, "/employee"):
			employee = append(employee, rt)
		default:
			public = append(public, rt)
		}
	}

	var b strings.Builder
	b.WriteString("# Phase 16 — Complete Route Inventory (item AH)\n\n")
	b.WriteString("Generated from the REAL production route table (`routes.RegisterRoutes`) by\n")
	b.WriteString("`TestGenerateRouteInventory` — not hand-maintained, so it cannot drift from\n")
	b.WriteString("the code.\n\n")
	b.WriteString("**IMPORTANT — inventory is not testing.** This document establishes 100%\n")
	b.WriteString("route INVENTORY. It does NOT claim these routes were security-tested.\n")
	b.WriteString("Phase 14 security-tested a representative subset; the Phase 16 report keeps\n")
	b.WriteString("INVENTORIED, SECURITY-TESTED and NOT-TESTED as separate counts.\n\n")

	fmt.Fprintf(&b, "## Totals\n\n")
	fmt.Fprintf(&b, "| Tier | Routes |\n|---|---|\n")
	fmt.Fprintf(&b, "| Public (unauthenticated) | %d |\n", len(public))
	fmt.Fprintf(&b, "| Customer JWT | %d |\n", len(customer))
	fmt.Fprintf(&b, "| Employee JWT | %d |\n", len(employee))
	fmt.Fprintf(&b, "| Admin JWT + RBAC permission | %d |\n", len(adminRoutes))
	fmt.Fprintf(&b, "| **TOTAL** | **%d** |\n\n", len(routes))

	section := func(title, note string, list []gin.RouteInfo) {
		fmt.Fprintf(&b, "## %s (%d)\n\n%s\n\n", title, len(list), note)
		b.WriteString("| Method | Path |\n|---|---|\n")
		for _, rt := range list {
			fmt.Fprintf(&b, "| %s | `%s` |\n", rt.Method, rt.Path)
		}
		b.WriteString("\n")
	}

	section("Public routes", "Unauthenticated by design. Each is individually rate-limited (see `internal/routes/routes.go`); the Razorpay webhook additionally verifies an HMAC signature inside its handler.", public)
	section("Customer-authenticated routes", "Require a valid customer JWT (`AuthMiddleware`). Resource ownership is enforced inside each handler.", customer)
	section("Employee-authenticated routes", "Require a valid employee JWT (`EmployeeAuthMiddleware`).", employee)
	section("Admin routes", "Require a valid admin JWT (`AdminAuthMiddleware`, which also re-checks session revocation and account status on every request), the blanket `admin.api` rate limit, AND an explicit `rbac.Permission` per route.", adminRoutes)

	path := "../../PHASE16_ROUTE_INVENTORY.md"
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	t.Logf("wrote route inventory: %d total routes (public=%d customer=%d employee=%d admin=%d)",
		len(routes), len(public), len(customer), len(employee), len(adminRoutes))
}

func isPublicAuth(path string) bool {
	switch path {
	case "/auth/send-otp", "/auth/verify-otp", "/auth/create-mpin", "/auth/login":
		return true
	}
	return false
}
